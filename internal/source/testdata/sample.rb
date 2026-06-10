# An HTTP server with configurable options.
#
# Supports graceful shutdown and middleware chaining.
#
# @deprecated Use NewServer instead.
# @since 1.0
class Server
  # @return [String] the address to listen on
  attr_accessor :addr

  # @return [Proc, nil] the HTTP handler
  attr_accessor :handler

  # Creates a new Server instance.
  #
  # @param addr [String] the address to listen on
  # @param handler [Proc, nil] the HTTP handler
  def initialize(addr, handler = nil)
    @addr = addr
    @handler = handler
  end

  # Starts the server on the configured address.
  #
  # @return [void]
  # @raise [IOError] if the server cannot bind
  def listen_and_serve
  end
end

# Creates a new Server instance with the given address.
#
# @param addr [String] the address to listen on
# @return [Server] a new server instance
def new_server(addr)
  Server.new(addr)
end
