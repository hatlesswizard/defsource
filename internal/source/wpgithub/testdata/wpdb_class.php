<?php
/**
 * WordPress database access abstraction class.
 *
 * @since 0.71
 */
class wpdb {
	/**
	 * Column metadata for the last query.
	 *
	 * @var array
	 */
	public $col_meta = array();

	/**
	 * Column metadata for the last query (second declaration – mirrors real wpdb).
	 *
	 * @var array
	 */
	public $col_meta = array();

	/**
	 * Database host.
	 *
	 * @var string
	 */
	public $dbhost;
}
