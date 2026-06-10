<?php
/**
 * An HTTP server with configurable options.
 *
 * Supports graceful shutdown and middleware chaining.
 *
 * @since 1.0
 * @deprecated Use NewServer instead.
 */
class Server {
    /**
     * The address to listen on.
     *
     * @var string
     */
    public $addr;

    /**
     * The HTTP handler.
     *
     * @var callable|null
     */
    private $handler;

    /**
     * Creates a new Server instance.
     *
     * @param string $addr The address to listen on.
     * @param callable|null $handler The HTTP handler.
     */
    public function __construct($addr, $handler = null) {
        $this->addr = $addr;
        $this->handler = $handler;
    }

    /**
     * Starts the server on the configured address.
     *
     * @return void
     * @throws \RuntimeException If the server cannot bind.
     */
    public function listenAndServe() {
    }
}

/**
 * Creates a new Server instance with the given address.
 *
 * @param string $addr The address to listen on.
 * @return Server A new server instance.
 */
function new_server($addr) {
    return new Server($addr);
}
