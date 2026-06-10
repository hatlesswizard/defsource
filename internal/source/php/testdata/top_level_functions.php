<?php
/**
 * Retrieve post data given a post ID or post object.
 *
 * See sanitize_post() for optional $filter values. Also, the parameter
 * $post, must be given as a variable, since it is passed by reference.
 *
 * @since 1.5.1
 *
 * @param int|WP_Post|null $post   Optional. Post ID or post object. Defaults to global $post.
 * @param string           $output Optional. The required return type. Defaults to OBJECT.
 * @param string           $filter Optional. Type of filter to apply.
 * @return WP_Post|array|null Type corresponding to $output on success or null on failure.
 */
function get_post( $post = null, string $output = OBJECT, string $filter = 'raw' ): WP_Post|array|null {
	return null;
}

/**
 * Retrieve the post status based on the post ID.
 *
 * @since 2.0.0
 *
 * @param int|WP_Post $post Optional. Post ID or post object.
 * @return string|false Post status on success, false on failure.
 */
function get_post_status( $post = null ): string|false {
	return false;
}
