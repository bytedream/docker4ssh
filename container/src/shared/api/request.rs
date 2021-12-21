use std::fmt::{Display, Formatter};
use serde::{Deserialize, Serialize};
use serde_repr::{Deserialize_repr, Serialize_repr};

use crate::shared::api::api::{API, Method, Request, Result};
use crate::shared::api::api::Method::POST;

#[derive(Deserialize)]
pub struct PingResponse {
    pub received: u128
}

pub struct PingRequest {
    request: Request
}

impl PingRequest {
    pub fn new() -> Self {
        PingRequest {
            request: Request::new(String::from("/ping"))
        }
    }
    pub fn request(&self, api: &mut API) -> Result<PingResponse> {
        let result: PingResponse = api.request_with_err(&self.request)?.body()?;
        Ok(result)
    }
}

pub struct ErrorRequest {
    request: Request
}

impl ErrorRequest {
    pub fn new() -> Self {
        ErrorRequest {
            request: Request::new(String::from("/error"))
        }
    }

    pub fn request(&self, api: &mut API) -> Result<()> {
        api.request_with_err(&self.request)?.body()?;
        // should never call Ok
        Ok(())
    }
}

#[derive(Deserialize)]
pub struct InfoResponse {
    pub container_id: String
}

pub struct InfoRequest {
    request: Request
}

impl InfoRequest {
    pub fn new() -> Self {
        InfoRequest{
            request: Request::new(String::from("/info"))
        }
    }

    pub fn request(&self, api: &mut API) -> Result<InfoResponse> {
        let result: InfoResponse = api.request_with_err(&self.request)?.body()?;
        Ok(result)
    }
}

#[derive(Debug, Serialize_repr, Deserialize_repr)]
#[repr(u8)]
pub enum ConfigRunLevel {
    User = 1,
    Container = 2,
    Forever = 3
}

impl Display for ConfigRunLevel {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:?}", self)
    }
}

#[derive(Debug, Serialize_repr, Deserialize_repr)]
#[repr(u8)]
pub enum ConfigNetworkMode {
    Off = 1,
    Full = 2,
    Host = 3,
    Docker = 4,
    None = 5
}

impl Display for ConfigNetworkMode {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        write!(f, "{:?}", self)
    }
}

#[derive(Deserialize)]
pub struct ConfigGetResponse {
    pub network_mode: ConfigNetworkMode,
    pub configurable: bool,
    pub run_level: ConfigRunLevel,
    pub startup_information: bool,
    pub exit_after: String,
    pub keep_on_exit: bool
}

pub struct ConfigGetRequest {
    request: Request
}

impl ConfigGetRequest {
    pub fn new() -> ConfigGetRequest {
        ConfigGetRequest{
            request: Request::new(String::from("/config"))
        }
    }

    pub fn request(&self, api: &mut API) -> Result<ConfigGetResponse> {
        let result: ConfigGetResponse = api.request_with_err(&self.request)?.body()?;
        Ok(result)
    }
}

#[derive(Serialize)]
pub struct ConfigPostBody {
    pub network_mode: Option<ConfigNetworkMode>,
    pub configurable: Option<bool>,
    pub run_level: Option<ConfigRunLevel>,
    pub startup_information: Option<bool>,
    pub exit_after: Option<String>,
    pub keep_on_exit: Option<bool>
}

pub struct ConfigPostRequest {
    request: Request,
    pub body: ConfigPostBody
}

impl ConfigPostRequest {
    pub fn new() -> ConfigPostRequest {
        let mut request = Request::new(String::from("/config"));
        request.set_method(Method::POST);

        ConfigPostRequest {
            request,
            body: ConfigPostBody{
                network_mode: None,
                configurable: None,
                run_level: None,
                startup_information: None,
                exit_after: None,
                keep_on_exit: None
            }
        }
    }

    pub fn request(&mut self, api: &mut API) -> Result<()> {
        self.request.set_body(serde_json::to_string(&self.body)?);
        api.request_with_err(&self.request)?;
        Ok(())
    }
}

#[derive(Deserialize)]
pub struct AuthGetResponse {
    pub user: String,
    pub has_password: bool
}

pub struct AuthGetRequest {
    request: Request
}

impl AuthGetRequest {
    pub fn new() -> AuthGetRequest {
        AuthGetRequest{
            request: Request::new(String::from("/auth"))
        }
    }

    pub fn request(&self, api: &mut API) -> Result<AuthGetResponse> {
        let result: AuthGetResponse = api.request_with_err(&self.request)?.body()?;
        Ok(result)
    }
}

#[derive(Serialize)]
pub struct AuthPostBody {
    pub user: Option<String>,
    pub password: Option<String>
}

pub struct AuthPostRequest {
    request: Request,
    pub body: AuthPostBody
}

impl AuthPostRequest {
    pub fn new() -> AuthPostRequest {
        let mut request = Request::new(String::from("/auth"));
        request.set_method(POST);

        AuthPostRequest {
            request,
            body: AuthPostBody{
                user: None,
                password: None
            }
        }
    }

    pub fn request(&mut self, api: &mut API) -> Result<()> {
        self.request.set_body(serde_json::to_string(&self.body)?);
        api.request_with_err(&self.request)?;
        Ok(())
    }
}
