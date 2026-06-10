# Enumerable provides collection methods.
#
# @since 1.0.0
module Enumerable
  # Returns the first n elements.
  #
  # @param n [Integer] number of elements
  # @return [Array] first n elements
  def first(n = nil)
    n ? take(n) : take(1).first
  end

  # Maps each element using the block.
  #
  # @param block [Proc] transformation block
  # @return [Array] transformed elements
  def map(&block)
    result = []
    each { |e| result << block.call(e) }
    result
  end

  # Selects elements matching the predicate.
  #
  # @param block [Proc] predicate block
  # @return [Array] matching elements
  def select(&block)
    result = []
    each { |e| result << e if block.call(e) }
    result
  end
end
