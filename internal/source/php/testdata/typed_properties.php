<?php
/**
 * Class with PHP 7.4+ typed properties.
 *
 * @since 5.5.0
 */
class WP_Block {
	/**
	 * Name of block.
	 *
	 * @var string
	 */
	public string $name;

	/**
	 * Original array of parsed block data.
	 *
	 * @var array
	 */
	public array $parsed_block;

	/**
	 * All available context of the current hierarchy.
	 *
	 * @var array
	 */
	public array $available_context;

	/**
	 * Block type registry.
	 *
	 * @var WP_Block_Type_Registry
	 */
	public WP_Block_Type_Registry $block_type_registry;

	/**
	 * Inner blocks.
	 *
	 * @var WP_Block_List
	 */
	public WP_Block_List $inner_blocks;
}
