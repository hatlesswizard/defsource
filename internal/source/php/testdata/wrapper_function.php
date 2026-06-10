<?php
/**
 * Retrieve the value of a transient.
 *
 * Wrapper around wp_cache_get that reads from the transients table.
 *
 * @since 2.8.0
 *
 * @param string $transient Transient name.
 * @return mixed Value of transient.
 */
function get_transient( string $transient ): mixed {
	return wp_cache_get( $transient, 'transient' );
}

/**
 * Does an action. Not a wrapper — has multiple statements.
 *
 * @since 1.2.0
 *
 * @param string $tag The name of the action.
 * @param mixed  ...$args Additional arguments passed to callbacks.
 */
function do_complex_action( string $tag, mixed ...$args ): void {
	$wp_filter = $GLOBALS['wp_filter'];
	if ( ! isset( $wp_filter[ $tag ] ) ) {
		return;
	}
	$all_args = func_get_args();
	_wp_call_all_hook( $all_args );
	$wp_filter[ $tag ]->do_action( $all_args );
}

/**
 * Fires a void wrapper (no return).
 *
 * @since 2.1.0
 *
 * @param string $hook The hook name.
 */
function fire_hook( string $hook ): void {
	do_action( $hook );
}
