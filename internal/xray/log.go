package xray

import xlog "github.com/xtls/xray-core/common/log"

// discardLogHandler 丢弃 xray-core 的全局日志。
type discardLogHandler struct{}

func (discardLogHandler) Handle(xlog.Message) {}

// init 关闭 xray-core 的全局日志(config 加载期的 deprecated/started 等 Warning)。
// 默认这些第三方日志会打到 stdout,污染 CLI 的 `-o json/csv | ...` 管道,且对本工具用户不可操作
// (节点成败已在结果里体现;每实例日志另在 buildJSONConfig 用 loglevel=none 关闭)。直接丢弃最干净,
// 同时保证 `-log-level silent` 真正安静。
func init() {
	xlog.RegisterHandler(discardLogHandler{})
}
