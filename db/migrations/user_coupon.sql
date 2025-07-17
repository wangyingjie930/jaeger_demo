CREATE TABLE `user_coupon` (
                               `id` BIGINT UNSIGNED NOT-NULL AUTO_INCREMENT COMMENT '自增主键',
                               `coupon_code` VARCHAR(64) NOT-NULL UNIQUE COMMENT '用户优惠券唯一编码, 用于核销',
                               `user_id` VARCHAR(64) NOT-NULL COMMENT '用户ID',
                               `template_id` BIGINT UNSIGNED NOT-NULL COMMENT '关联的模板ID',
                               `status` TINYINT UNSIGNED NOT-NULL DEFAULT 1 COMMENT '状态: 1-未使用, 2-已使用, 3-已过期, 4-已冻结(SAGA)',
                               `order_id` VARCHAR(64) DEFAULT NULL COMMENT '核销时关联的订单ID',
                               `valid_from` TIMESTAMP NOT-NULL COMMENT '有效期开始时间',
                               `valid_to` TIMESTAMP NOT-NULL COMMENT '有效期结束时间',
                               `used_at` TIMESTAMP NULL DEFAULT NULL COMMENT '使用时间',
                               `created_at` TIMESTAMP NOT-NULL DEFAULT CURRENT_TIMESTAMP,
                               `updated_at` TIMESTAMP NOT-NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
                               PRIMARY KEY (`id`),
                               INDEX `idx_user_coupon` (`user_id`, `status`, `valid_to`),
                               INDEX `idx_coupon_code` (`coupon_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户优惠券表';