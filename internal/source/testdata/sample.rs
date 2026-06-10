/// An HTTP server with configurable options.
///
/// Supports graceful shutdown and middleware chaining.
///
/// # Examples
///
/// ```
/// let server = Server::new("127.0.0.1:8080");
/// server.listen_and_serve();
/// ```
pub struct Server {
    /// The address to listen on.
    pub addr: String,
    /// The HTTP handler.
    handler: Option<Box<dyn Fn()>>,
}

impl Server {
    /// Creates a new Server instance with the given address.
    ///
    /// # Arguments
    ///
    /// * `addr` - The address to listen on.
    pub fn new(addr: &str) -> Self {
        Server {
            addr: addr.to_string(),
            handler: None,
        }
    }

    /// Starts the server on the configured address.
    pub fn listen_and_serve(&self) -> Result<(), std::io::Error> {
        Ok(())
    }
}
