<?php
/**
 * WordPress Object Cache functions.
 *
 * @since 2.0.0
 */

/**
 * Retrieves the cache contents from the cache by key and group.
 *
 * @since 2.0.0
 *
 * @param int|string $key    The key under which the cache contents are stored.
 * @param string     $group  Optional. Where the cache contents are grouped. Default empty.
 * @param bool       $force  Optional. Whether to force an update of the local cache. Default false.
 * @param bool       $found  Optional. Whether the key was found in the cache (passed by reference).
 * @return mixed|false The cache contents on success, false on failure.
 */
function wp_cache_get( $key, string $group = '', bool $force = false, bool &$found = null ): mixed {
	return false;
}

/**
 * Adds data to the cache.
 *
 * @since 2.0.0
 *
 * @param int|string $key    The cache key.
 * @param mixed      $data   The data to add to the cache.
 * @param string     $group  Optional. The group under which to store the data.
 * @param int        $expire Optional. Expiration in seconds.
 * @return bool True on success, false if cache key and group already exist.
 */
function wp_cache_add( $key, mixed $data, string $group = '', int $expire = 0 ): bool {
	return true;
}
