<?php
/**
 * Manages queries for WordPress.
 *
 * @since 1.5.0
 */
class WP_Query {
	/**
	 * Query variables.
	 *
	 * @var array
	 */
	public $query_vars = array();

	/**
	 * Current post.
	 *
	 * @var int
	 */
	public $current_post = -1;

	/**
	 * Total found posts.
	 *
	 * @var int
	 */
	protected $found_posts = 0;

	/**
	 * Initiates object properties and sets up default values.
	 *
	 * @since 1.5.0
	 */
	public function init(): void {
		$this->query_vars = array();
	}

	/**
	 * Retrieve the posts based on query variables.
	 *
	 * @since 1.5.0
	 *
	 * @return WP_Post[]|int[] Array of post objects or post IDs.
	 */
	public function get_posts(): array {
		return array();
	}

	/**
	 * Get a query variable.
	 *
	 * @since 1.5.0
	 *
	 * @param string $query_var Query variable key.
	 * @param mixed  $default   Optional. Value to return if the query variable is not set.
	 * @return mixed Contents of the query variable.
	 */
	public function get( string $query_var, mixed $default = '' ): mixed {
		return $this->query_vars[ $query_var ] ?? $default;
	}

	/**
	 * Gets all posts by calling the internal method.
	 *
	 * @since 2.0.0
	 *
	 * @return WP_Post[]|int[] Array of posts.
	 */
	public static function fetch_all(): array {
		return self::get_posts_static();
	}
}

/**
 * Retrieves a list of post objects.
 *
 * @since 1.2.0
 *
 * @param array $args Optional. Arguments to retrieve posts.
 * @return WP_Post[] Array of post objects.
 */
function get_posts( array $args = array() ): array {
	return array();
}
