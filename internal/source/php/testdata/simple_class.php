<?php
/**
 * Represents a WordPress post.
 *
 * Core class used to implement the WP_Post object. This class provides access
 * to post data and helper methods.
 *
 * @since 3.5.0
 */
class WP_Post {
	/**
	 * The post ID.
	 *
	 * @var int
	 */
	public $ID;

	/**
	 * The post title.
	 *
	 * @var string
	 */
	public $post_title = '';

	/**
	 * Retrieves a WP_Post instance.
	 *
	 * @since 3.5.0
	 *
	 * @param int $post_id Post ID.
	 * @return WP_Post|false Post object, false otherwise.
	 */
	public static function get_instance( $post_id ) {
		return false;
	}

	/**
	 * Gets the post ID.
	 *
	 * @since 3.5.0
	 *
	 * @return int The post ID.
	 */
	public function get_id(): int {
		return $this->ID;
	}
}
