pub mod sync;

use thiserror::Error;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ApiErrorKind {
    Transient,
    RateLimited,
    Unauthorized,
    Forbidden,
    NotFound,
    Conflict,
    Validation,
    Server,
    Unknown,
}

#[derive(Debug, Error)]
#[error("{kind:?}: {message}")]
pub struct ApiError {
    kind: ApiErrorKind,
    status: Option<u16>,
    message: String,
}

impl ApiError {
    pub fn from_reqwest(source: reqwest::Error) -> Self {
        let status = source.status().map(|status| status.as_u16());
        let kind = match status {
            None => ApiErrorKind::Transient,
            Some(400 | 422) => ApiErrorKind::Validation,
            Some(401) => ApiErrorKind::Unauthorized,
            Some(403) => ApiErrorKind::Forbidden,
            Some(404) => ApiErrorKind::NotFound,
            Some(408 | 425) => ApiErrorKind::Transient,
            Some(409) => ApiErrorKind::Conflict,
            Some(429) => ApiErrorKind::RateLimited,
            Some(status) if status >= 500 => ApiErrorKind::Server,
            _ => ApiErrorKind::Unknown,
        };

        Self {
            kind,
            status,
            message: source.to_string(),
        }
    }

}

#[cfg(test)]
impl ApiError {
    pub fn for_test(kind: ApiErrorKind) -> Self {
        let status = match kind {
            ApiErrorKind::Validation => Some(400),
            ApiErrorKind::Unauthorized => Some(401),
            ApiErrorKind::Forbidden => Some(403),
            ApiErrorKind::NotFound => Some(404),
            ApiErrorKind::Conflict => Some(409),
            ApiErrorKind::RateLimited => Some(429),
            ApiErrorKind::Server => Some(500),
            ApiErrorKind::Transient | ApiErrorKind::Unknown => None,
        };
        Self {
            kind,
            status,
            message: format!("test {kind:?}"),
        }
    }
}

pub type ApiResult<T> = Result<T, ApiError>;
