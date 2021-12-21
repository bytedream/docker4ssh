use std::fs;
use std::process::exit;
use log::{LevelFilter, error};
use docker4ssh::configure::cli;
use docker4ssh::shared::logging::init_logger;

fn main() {
    if init_logger(LevelFilter::Debug).is_err() {
        println!("Failed to initialize logger");
    }

    match fs::read_to_string("/etc/docker4ssh") {
        Ok(route) => cli(route),
        Err(e) => {
            error!("Failed to read /etc/docker4ssh: {}", e.to_string());
            exit(1);
        }
    }
}
