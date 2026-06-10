#pragma once

#include <string>
#include <functional>
#include <stdexcept>

/**
 * @brief An HTTP server with configurable options.
 *
 * Supports graceful shutdown and middleware chaining.
 *
 * @since 1.0
 * @deprecated Use NewServer() instead.
 */
class Server {
public:
    /// The address to listen on.
    std::string addr;

    /**
     * @brief Creates a new Server instance.
     *
     * @param addr The address to listen on.
     * @param handler The HTTP handler.
     */
    Server(const std::string& addr, std::function<void()> handler = nullptr)
        : addr(addr), handler_(handler) {}

    /**
     * @brief Starts the server on the configured address.
     *
     * @throws std::runtime_error If the server cannot bind.
     */
    void listenAndServe() {}

private:
    std::function<void()> handler_;
};

/**
 * @brief Creates a new Server instance with the given address.
 *
 * @param addr The address to listen on.
 * @return A new Server instance.
 */
Server newServer(const std::string& addr) {
    return Server(addr);
}
