# A sample ActiveRecord-like base class for testing.
#
# @since 1.0.0
# @deprecated Use NewBase instead
module ActiveRecord
  # Base class for all models.
  #
  # Provides database access, validations, and callbacks.
  #
  # @example
  #   class User < ActiveRecord::Base
  #     validates :name, presence: true
  #   end
  class Base
    include Comparable
    extend ClassMethods
    prepend Callbacks

    attr_reader :id, :created_at
    attr_writer :updated_at
    attr_accessor :name, :email

    # The connection pool used by this model.
    POOL_SIZE = 5

    # Finds a record by its primary key.
    #
    # @param id [Integer] the primary key value
    # @return [Base] the found record
    # @raise [RecordNotFound] if no record is found
    # @since 1.0.0
    def self.find(id)
      connection.find(id)
    end

    # Creates a new record with the given attributes.
    #
    # @param attributes [Hash] the attributes to set
    # @return [Base] the new record
    def self.create(attributes = {})
      new(attributes).tap(&:save)
    end

    # Initializes a new record.
    #
    # @param attributes [Hash] initial attribute values
    # @param block [Proc] optional block for configuration
    def initialize(attributes = {}, &block)
      @attributes = attributes
      yield self if block_given?
    end

    # Saves the record to the database.
    #
    # @return [Boolean] true if saved successfully
    def save
      run_callbacks(:save) { persist }
    end

    # Updates multiple attributes at once.
    #
    # @param attrs [Hash] attributes to update
    # @return [Boolean] true if updated successfully
    def update(attrs = {})
      assign_attributes(attrs)
      save
    end

    protected

    # Runs validation callbacks.
    def validate!
      run_callbacks(:validate)
    end

    private

    # Persists the record to the database.
    def persist
      connection.insert(self)
    end

    # Assigns attributes from a hash.
    #
    # @param attrs [Hash] the attributes
    def assign_attributes(attrs)
      attrs.each { |k, v| send("#{k}=", v) }
    end
  end

  # Query interface for building database queries.
  class Relation
    attr_reader :klass, :values

    # Creates a new Relation.
    #
    # @param klass [Class] the model class
    # @param values [Hash] initial query values
    def initialize(klass, values = {})
      @klass = klass
      @values = values
    end

    # Adds a WHERE condition.
    #
    # @param conditions [Hash] the conditions
    # @return [Relation] a new relation with the condition added
    def where(conditions)
      spawn.tap { |r| r.values[:where] = conditions }
    end

    # Limits the number of results.
    #
    # @param count [Integer] maximum number of results
    # @return [Relation] a new relation with the limit set
    def limit(count)
      spawn.tap { |r| r.values[:limit] = count }
    end

    private

    def spawn
      self.class.new(@klass, @values.dup)
    end
  end
end
