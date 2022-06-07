use std::collections::HashMap;
use std::io::{Read, Write};
use std::net::TcpStream;
use anyhow::{anyhow, bail, Result};
use serde::Deserialize;

pub struct API {
    route: String,
}

impl API {
    pub const fn new(route: String) -> Self {
        API {
            route,
        }
    }

    pub fn new_connection(&mut self) -> Result<TcpStream> {
        match TcpStream::connect(&self.route) {
            Ok(stream) => Ok(stream),
            Err(e) => bail!("Failed to connect to {}: {}", self.route, e.to_string())
        }
    }

    pub fn request(&mut self, request: &Request) -> Result<APIResult> {
        let mut connection = self.new_connection()?;

        connection.write_all(request.as_string().as_bytes())?;
        let mut buf: String = String::new();
        connection.read_to_string(&mut buf)?;
        Ok(APIResult::new(request, buf))
    }

    pub fn request_with_err(&mut self, request: &Request) -> Result<APIResult> {
        let result = self.request(request)?;
        if result.result_code >= 400 {
            let err: APIError = result.body()?;
            bail!("Error {}: {}", result.result_code, err.message)
        } else {
            Ok(result)
        }
    }
}

#[derive(Deserialize)]
pub struct APIError {
    message: String
}

pub struct APIResult {
    // TODO: Store the whole request instead of only the path
    request_path: String,

    result_code: i32,
    result_body: String
}

impl APIResult {
    fn new(request: &Request, raw_response: String) -> Self {
        APIResult {
            request_path: request.path.clone(),

            // TODO: Parse http body better
            result_code: raw_response[9..12].parse().unwrap(),
            result_body: raw_response.split_once("\r\n\r\n").unwrap().1.to_string()
        }
    }

    pub fn path(self) -> String {
        self.request_path
    }

    pub fn code(&self) -> i32 {
        return self.result_code
    }

    pub fn has_body(&self) -> bool {
        self.result_body.len() > 0
    }

    pub fn body<'a, T: Deserialize<'a>>(&'a self) -> Result<T> {
        let result = serde_json::from_str(&self.result_body).map_err(|e| {
            // checks if the error has a body and if so, return it
            if self.has_body() {
                let error: APIError = serde_json::from_str(&self.result_body).unwrap_or_else(|_| {
                    APIError{message: format!("could not deserialize response: {}", e.to_string())}
                });
                anyhow!("Failed to call '{}': {}", self.request_path, error.message)
            } else {
                anyhow!("Failed to call '{}': {}", self.request_path, e.to_string())
            }
        })?;
        Ok(result)
    }
}

pub enum Method {
    GET,
    POST
}

pub struct Request {
    method: Method,
    path: String,
    headers: HashMap<String, String>,
    body: String,
}

impl Request {
    pub fn new(path: String) -> Self {
        Request{
            method: Method::GET,
            path,
            headers: Default::default(),
            body: "".to_string(),
        }
    }

    pub fn set_method(&mut self, method: Method) -> &Self {
        self.method = method;
        self
    }

    pub fn set_path(&mut self, path: String) -> &Self {
        self.path = path;
        self
    }

    pub fn set_header(&mut self, field: &str, value: String) -> &Self {
        self.headers.insert(String::from(field), value);
        self
    }

    pub fn set_body(&mut self, body: String) -> &Self {
        self.body = body;
        self.headers.insert("Content-Length".to_string(), self.body.len().to_string());
        self
    }

    pub fn as_string(&self) -> String {
        let method;
        match self.method {
            Method::GET => method = "GET",
            Method::POST => method = "POST"
        }

        let headers_as_string = self.headers.iter().map(|f| format!("{}: {}", f.0, f.1)).collect::<String>();

        return format!("{} {} HTTP/1.0\r\n\
        {}\r\n\r\n\
        {}\r\n", method, self.path, headers_as_string, self.body)
    }
}
