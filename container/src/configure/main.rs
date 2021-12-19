use std::fs;
use std::net::TcpStream;
use std::os::unix::net::UnixStream;
use std::process::exit;
use log::{LevelFilter, trace, warn, info, error};
use docker4ssh::configure::cli;
use docker4ssh::shared::logging::init_logger;

fn main() {
    init_logger(LevelFilter::Debug);

    match fs::read_to_string("/etc/docker4ssh") {
        Ok(route) => cli(route),
        Err(e) => {
            error!("Failed to read /etc/docker4ssh: {}", e.to_string());
            exit(1);
        }
    }
}
