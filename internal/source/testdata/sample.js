/**
 * An HTTP server with configurable options.
 *
 * Supports graceful shutdown and middleware chaining.
 *
 * @deprecated Use NewServer instead.
 * @since 1.0
 */
class Server {
    /**
     * Creates a new Server instance.
     *
     * @param {string} addr - The address to listen on.
     * @param {Function} [handler] - The HTTP handler.
     */
    constructor(addr, handler) {
        this.addr = addr;
        this.handler = handler;
    }

    /**
     * Starts the server on the configured address.
     *
     * @returns {Promise<void>} Resolves when server starts.
     * @throws {Error} If the server cannot bind.
     */
    async listenAndServe() {
    }
}

/**
 * Creates a new Server instance with the given address.
 *
 * @param {string} addr - The address to listen on.
 * @returns {Server} A new server instance.
 */
function newServer(addr) {
    return new Server(addr);
}

module.exports = { Server, newServer };
