use std::fmt::{Debug, format};
use std::net::TcpStream;
use std::os::unix::process::ExitStatusExt;
use std::process::{Command, ExitStatus};
use std::time::SystemTime;
use log::{info, warn};
use structopt::StructOpt;
use structopt::clap::AppSettings;
use crate::configure::cli::parser;
use crate::shared::api::api::API;
use crate::shared::api::request;
use crate::shared::api::request::{ConfigGetResponse, ConfigNetworkMode, ConfigPostRequest, ConfigRunLevel};

type Result<T> = std::result::Result<T, failure::Error>;

trait Execute {
    fn execute(self, api: &mut API) -> Result<()>;
}

#[derive(StructOpt)]
#[structopt(
    name = "configure",
    about = "A command line wrapper to control docker4ssh containers from within them",
    settings = &[AppSettings::ArgRequiredElseHelp]
)]
struct Opts {
    #[structopt(short, long, global = true, help = "Verbose output")]
    verbose: bool,

    #[structopt(subcommand)]
    commands: Option<Root>
}

#[derive(StructOpt)]
#[structopt(
    name = "ping",
    about = "Ping the control socket"
)]
struct Ping {}

impl Execute for Ping {
    fn execute(self, api: &mut API) -> Result<()> {
        let start = SystemTime::now().duration_since(SystemTime::UNIX_EPOCH)?.as_nanos();
        let result = request::PingRequest::new().request(api)?;
        info!("Pong! Ping is {:.4}ms", ((result.received - start) as f64) / 1000.0 / 1000.0);
        Ok(())
    }
}

#[derive(StructOpt)]
#[structopt(
    name = "error",
    about = "Example error message sent from socket",
)]
struct Error {}

impl Execute for Error {
    fn execute(self, api: &mut API) -> Result<()> {
        request::ErrorRequest::new().request(api)?;
        Ok(())
    }
}

#[derive(StructOpt)]
#[structopt(
    name = "info",
    about = "Shows information about the current container",
)]
struct Info {}

impl Execute for Info {
    fn execute(self, api: &mut API) -> Result<()> {
        let result = request::InfoRequest:: new().request(api)?;
        info!(concat!(
            "\tContainer ID: {}"
        ), result.container_id);
        Ok(())
    }
}

#[derive(StructOpt)]
#[structopt(
    name = "config",
    about = "Get or set the behavior of the current container",
    settings = &[AppSettings::ArgRequiredElseHelp]
)]
struct Config {
    #[structopt(subcommand)]
    commands: Option<ConfigCommands>
}

#[derive(StructOpt)]
enum ConfigCommands {
    Get(ConfigGet),
    Set(ConfigSet)
}

#[derive(StructOpt)]
#[structopt(
    name = "get",
    about = "Show the current container behavior"
)]
struct ConfigGet {}

impl Execute for ConfigGet {
    fn execute(self, api: &mut API) -> Result<()> {
        let response: ConfigGetResponse = request::ConfigGetRequest::new().request(api)?;

        info!(concat!(
            "\tNetwork Mode: {}\n",
            "\tConfigurable: {}\n",
            "\tRun Level: {}\n",
            "\tStartup Information: {}\n",
            "\tExit After: {}\n",
            "\tKeep On Exit: {}"
        ), response.network_mode, response.configurable, response.run_level, response.startup_information, response.exit_after, response.keep_on_exit);

        Ok(())
    }
}

#[derive(StructOpt)]
#[structopt(
    name = "set",
    about = "Set the current container behavior",
    settings = &[AppSettings::ArgRequiredElseHelp]
)]
struct ConfigSet {
    #[structopt(long, help = "If the container should keep running even after the user exits", parse(try_from_str = parser::parse_network_mode))]
    network_mode: Option<ConfigNetworkMode>,

    #[structopt(long, help = "If the container should be configurable from within")]
    configurable: Option<bool>,

    #[structopt(long, help = "Set the container stop behavior", parse(try_from_str = parser::parse_config_run_level))]
    run_level: Option<ConfigRunLevel>,

    #[structopt(long, help = "If information about the container should be shown when a user connects")]
    startup_information: Option<bool>,

    #[structopt(long, help = "Process name after which the container should exit")]
    exit_after: Option<String>,

    #[structopt(long, help = "If the container should be not deleted after exit")]
    keep_on_exit: Option<bool>
}

impl Execute for ConfigSet {
    fn execute(self, api: &mut API) -> Result<()> {
        let mut request = request::ConfigPostRequest::new();

        if let Some(exit_after) = self.exit_after.as_ref() {
            let program_runs = Command::new("pidof")
                .arg("-s")
                .arg(exit_after).status().unwrap().success();
            if !program_runs {
                warn!("NOTE: There is currently no process running with the name '{}'", exit_after);
            }
        }

        request.body.network_mode = self.network_mode;
        request.body.configurable = self.configurable;
        request.body.run_level = self.run_level;
        request.body.startup_information = self.startup_information;
        request.body.exit_after = self.exit_after;
        request.body.keep_on_exit = self.keep_on_exit;

        request.request(api)?;

        if let Some(keep_on_exit) = self.keep_on_exit {
            if keep_on_exit {
                if let Ok(auth) = request::AuthGetRequest::new().request(api) {
                    info!("To reconnect to this container, use the user '{}' for the ssh connection", &auth.user)
                }
            }
        }

        Ok(())
    }
}

#[derive(StructOpt)]
#[structopt(
    name = "auth",
    about = "Get or set the container authentication",
    settings = &[AppSettings::ArgRequiredElseHelp]
)]
struct Auth {
    #[structopt(subcommand)]
    commands: Option<AuthCommands>
}

#[derive(StructOpt)]
enum AuthCommands {
    Get(AuthGet),
    Set(AuthSet)
}

#[derive(StructOpt)]
#[structopt(
    name = "get",
    about = "Show the current username used for ssh authentication and if a password is set"
)]
struct AuthGet {}

impl Execute for AuthGet {
    fn execute(self, api: &mut API) -> Result<()> {
        let response = request::AuthGetRequest::new().request(api)?;

        info!(concat!(
            "\tUser: {}\n",
            "\tHas Password: {}\n"
        ), response.user, response.has_password);

        Ok(())
    }
}

#[derive(StructOpt)]
    #[structopt(
    name = "set",
    about = "Set the authentication settings",
    settings = &[AppSettings::ArgRequiredElseHelp]
)]
struct AuthSet {
    #[structopt(long, help = "The container username")]
    user: Option<String>,
    #[structopt(long, help = "The container password. If empty, the authentication gets removed")]
    password: Option<String>
}

impl Execute for AuthSet {
    fn execute(self, api: &mut API) -> Result<()> {
        let mut request = request::AuthPostRequest::new();
        request.body.user = self.user;
        request.body.password = self.password.clone();

        request.request(api)?;

        if let Some(password) = self.password {
            if password == "" {
                warn!("No password was specified so the authentication got deleted")
            }
        }

        Ok(())
    }
}

#[derive(StructOpt)]
enum Root {
    Auth(Auth),
    Error(Error),
    Info(Info),
    Ping(Ping),
    Config(Config)
}

pub fn cli(route: String) {
    if let Some(subcommand) = Opts::from_args().commands {
        let mut result: Result<()> = Ok(());
        let mut api = API::new(route, String::new());
        match subcommand {
            Root::Auth(auth) => {
                if let Some(subsubcommand) = auth.commands {
                    match subsubcommand {
                        AuthCommands::Get(auth_get) => {
                            result = auth_get.execute(&mut api)
                        }
                        AuthCommands::Set(auth_set) => {
                            result = auth_set.execute(&mut api)
                        }
                    }
                }
            },
            Root::Error(error) => result = error.execute(&mut api),
            Root::Info(info) => result = info.execute(&mut api),
            Root::Ping(ping) => result = ping.execute(&mut api),
            Root::Config(config) => {
                if let Some(subsubcommand) = config.commands {
                    match subsubcommand {
                        ConfigCommands::Get(config_get) => {
                            result = config_get.execute(&mut api)
                        }
                        ConfigCommands::Set(config_set) => {
                            result = config_set.execute(&mut api)
                        }
                    }
                }
            }
        }
        if result.is_err() {
            log::error!("{}", result.err().unwrap().to_string())
        }
    }
}
