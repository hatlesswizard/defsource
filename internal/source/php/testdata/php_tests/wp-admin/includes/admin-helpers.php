<?php
/**
 * Checks if the user is an administrator.
 *
 * @since 4.1.0
 *
 * @param int $user_id Optional. User ID. Defaults to current user.
 * @return bool True if user is an administrator, false otherwise.
 */
function current_user_can_admin( int $user_id = 0 ): bool {
	return current_user_can( 'manage_options' );
}
