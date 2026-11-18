# 项目界面截图

这些 PNG 是从本仓库前端代码（截图基准提交 `02c3b9d`）在本地运行后截取的真实 UI，不是重绘的产品原型。

为避免向公开仓库上传敏感数据，浏览器中的 API 请求全部使用本地演示响应，并阻止访问外部站点；没有连接生产后端或发起真实支付。站点品牌文字统一替换为 `FDC Store Demo`，商品封面使用无第三方品牌的演示图。

## 图片列表

| 图片 | 展示内容 |
| --- | --- |
| [storefront-catalog.png](storefront-catalog.png) | 商品分类、商品卡片、交付方式、IDR 价格 |
| [storefront-product.png](storefront-product.png) | 桌面端商品详情、SKU、库存、购买入口 |
| [storefront-mobile.png](storefront-mobile.png) | 同一商品的移动端布局 |
| [admin-payssion.png](admin-payssion.png) | Payssion 支付渠道及手续费、币种配置 |
| [admin-support.png](admin-support.png) | 客服会话、快捷回复和关联订单 |

## 隐私处理

- 客户邮箱全部使用 `demo-*@example.invalid`，不是真实联系方式。
- 订单号全部使用 `DEMO-ORDER-*`，不关联真实交易。
- 商品名称、价格、库存、用户 ID、聊天内容和时间均为虚构演示数据。
- 支付 API Key 使用 `DEMO-NOT-A-REAL-KEY` 占位值，Secret Key 使用掩码；没有嵌入真实凭证。
- 截图不包含浏览器地址栏、生产域名、IP、后台隐藏路径、支付二维码或实际交付卡密。
- 输出为重新编码的 PNG，移除文本、EXIF 等非必要元数据。

演示截图不能作为实际销量、真实支付成功、自动发货成功或第三方服务可用性的证明。项目基于开源 Dujiao-Next 二次开发，功能归属与改动范围以仓库主 README 为准。
