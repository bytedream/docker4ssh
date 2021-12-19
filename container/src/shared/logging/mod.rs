use log::{LevelFilter, SetLoggerError};

pub mod logger;

pub use logger::Logger;

static LOGGER: Logger = Logger;

pub fn init_logger(level: LevelFilter) -> Result<(), SetLoggerError> {
    log::set_logger(&Logger).map(|()| log::set_max_level(level))
}
