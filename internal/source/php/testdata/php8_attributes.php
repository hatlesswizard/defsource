<?php
/**
 * Handles the REST API request.
 *
 * @since 5.6.0
 *
 * @param WP_REST_Request $request The request object.
 * @return WP_REST_Response The response.
 */
#[Route('/wp/v2/posts')]
function handle_request( WP_REST_Request $request ): WP_REST_Response {
	return new WP_REST_Response();
}

/**
 * Sanitizes a string key.
 *
 * Keys are used as identifiers for various things. Lowercase alphanumeric
 * and hyphens are allowed.
 *
 * @since 3.0.0
 *
 * @param string $key String key.
 * @return string Sanitized key.
 */
#[Pure]
#[AllowDynamicProperties]
function sanitize_key( string $key ): string {
	return $key;
}
