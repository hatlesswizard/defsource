module Delegation
  class Proxy
    # A simple wrapper method that delegates to target.
    def fetch(key)
      @target.fetch(key)
    end

    # A wrapper that delegates to self
    def process(data)
      self.internal_process(data)
    end

    # NOT a wrapper - multiple statements
    def complex_method(x)
      y = transform(x)
      finalize(y)
    end

    # A splat-forwarding wrapper
    def forward(*args)
      target(*args)
    end

    # A full delegation pattern
    def delegate_all(*args, **opts, &block)
      @backend.call(*args, **opts, &block)
    end
  end
end
