"""Sample module for testing."""


class Server:
    """An HTTP server with configurable options.

    Supports graceful shutdown and middleware chaining.

    Deprecated:
        Use NewServer instead.

    Args:
        addr (str): The address to listen on.
        handler (callable): The HTTP handler.
    """

    def __init__(self, addr: str, handler=None):
        """Initialize the server.

        Args:
            addr (str): The address to listen on.
            handler (callable, optional): The HTTP handler.
        """
        self.addr = addr
        self.handler = handler

    def listen_and_serve(self) -> None:
        """Start the server on the configured address.

        Returns:
            None
        """
        pass


def new_server(addr: str) -> Server:
    """Create a new Server instance with the given address.

    Args:
        addr (str): The address to listen on.

    Returns:
        Server: A new server instance.
    """
    return Server(addr)
