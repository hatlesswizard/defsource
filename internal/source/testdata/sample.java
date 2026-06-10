package com.example;

/**
 * An HTTP server with configurable options.
 *
 * Supports graceful shutdown and middleware chaining.
 *
 * @since 1.0
 * @deprecated Use NewServer instead.
 */
public class Server {
    /** The address to listen on. */
    private String addr;

    /** The HTTP handler. */
    private Object handler;

    /**
     * Creates a new Server instance with the given address.
     *
     * @param addr The address to listen on.
     */
    public Server(String addr) {
        this.addr = addr;
    }

    /**
     * Starts the server on the configured address.
     *
     * @throws IOException if the server cannot bind.
     */
    public void listenAndServe() throws IOException {
    }
}
