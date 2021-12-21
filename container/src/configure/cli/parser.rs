use crate::shared::api::request::{ConfigNetworkMode, ConfigRunLevel};

pub fn parse_network_mode(src: &str) -> Result<ConfigNetworkMode, String> {
    match String::from(src).to_lowercase().as_str() {
        "off" | "1" => Ok(ConfigNetworkMode::Off),
        "full" | "2" => Ok(ConfigNetworkMode::Full),
        "host" | "3" => Ok(ConfigNetworkMode::Host),
        "docker" | "4" => Ok(ConfigNetworkMode::Docker),
        "none" | "5" => Ok(ConfigNetworkMode::None),
        _ => Err(format!("'{} is not a valid network mode. Choose from 'off', 'full', 'host', 'docker', 'none'", src))
    }
}

pub fn parse_config_run_level(src: &str) -> Result<ConfigRunLevel, String> {
    match String::from(src).to_lowercase().as_str() {
        "user" | "1" => Ok(ConfigRunLevel::User),
        "container" | "2" => Ok(ConfigRunLevel::Container),
        "forever" | "3" => Ok(ConfigRunLevel::Forever),
        _ => Err(format!("'{}' is not a valid run level. Choose from: 'user', 'container', 'forever'", src))
    }
}
