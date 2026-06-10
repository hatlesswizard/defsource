/**
 * @file sample.c
 * @brief Sample C header for testing.
 */

#ifndef SAMPLE_H
#define SAMPLE_H

/**
 * @brief An HTTP server with configurable options.
 *
 * Supports graceful shutdown and middleware chaining.
 *
 * @since 1.0
 * @deprecated Use new_server() instead.
 */
typedef struct Server {
    /** The address to listen on. */
    const char* addr;
    /** The HTTP handler callback. */
    void (*handler)(void);
} Server;

/**
 * @brief Creates a new Server instance with the given address.
 *
 * @param addr The address to listen on.
 * @return A pointer to the new Server, or NULL on failure.
 */
Server* new_server(const char* addr);

/**
 * @brief Starts the server on the configured address.
 *
 * @param server The server to start.
 * @return 0 on success, non-zero on failure.
 */
int listen_and_serve(Server* server);

#endif /* SAMPLE_H */
