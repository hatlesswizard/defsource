using System;
using System.Threading.Tasks;

namespace Example
{
    /// <summary>
    /// An HTTP server with configurable options.
    /// Supports graceful shutdown and middleware chaining.
    /// </summary>
    /// <remarks>
    /// Use NewServer factory method for construction.
    /// </remarks>
    public class Server
    {
        /// <summary>The address to listen on.</summary>
        public string Addr { get; set; }

        /// <summary>The HTTP handler.</summary>
        private Action Handler { get; set; }

        /// <summary>
        /// Creates a new Server instance.
        /// </summary>
        /// <param name="addr">The address to listen on.</param>
        /// <param name="handler">The HTTP handler.</param>
        public Server(string addr, Action handler = null)
        {
            Addr = addr;
            Handler = handler;
        }

        /// <summary>
        /// Starts the server on the configured address.
        /// </summary>
        /// <returns>A task representing the async operation.</returns>
        /// <exception cref="InvalidOperationException">If the server cannot bind.</exception>
        public async Task ListenAndServeAsync()
        {
        }
    }
}
