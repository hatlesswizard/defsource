<?php
/**
 * Gets the length of a string using the PHP built-in strlen.
 *
 * This is a direct wrapper around strlen().
 *
 * @since 1.0.0
 *
 * @param string $str The string.
 * @return int The length of the string.
 */
function wp_strlen( string $str ): int {
	return strlen( $str );
}
