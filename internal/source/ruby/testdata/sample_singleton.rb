module Configuration
  # Registry for configuration entries.
  class Registry
    attr_reader :entries

    # Singleton class methods via class << self
    class << self
      # Returns the global instance.
      #
      # @return [Registry] the singleton instance
      def instance
        @instance ||= new
      end

      # Resets the global instance.
      def reset!
        @instance = nil
      end
    end

    # Registers a new entry.
    #
    # @param key [Symbol] the entry key
    # @param value [Object] the entry value
    def register(key, value)
      @entries ||= {}
      @entries[key] = value
    end

    # Also singleton method via def self.method
    def self.configure(options = {})
      instance.tap do |i|
        options.each { |k, v| i.register(k, v) }
      end
    end
  end
end
