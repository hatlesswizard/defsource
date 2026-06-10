/**
 * An HTTP server with configurable options.
 *
 * Supports graceful shutdown and middleware chaining.
 *
 * @deprecated Use NewServer instead.
 * @since 1.0
 */
export class Server {
    /** The address to listen on. */
    public addr: string;

    /** The HTTP handler. */
    private handler?: () => void;

    /**
     * Creates a new Server instance.
     *
     * @param {string} addr - The address to listen on.
     * @param {Function} [handler] - The HTTP handler.
     */
    constructor(addr: string, handler?: () => void) {
        this.addr = addr;
        this.handler = handler;
    }

    /**
     * Starts the server on the configured address.
     *
     * @returns {Promise<void>} Resolves when server starts.
     */
    async listenAndServe(): Promise<void> {
    }
}

/**
 * Creates a new Server instance with the given address.
 *
 * @param {string} addr - The address to listen on.
 * @returns {Server} A new server instance.
 */
export function newServer(addr: string): Server {
    return new Server(addr);
}
