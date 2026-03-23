package proxysql

import (
	"sync"

	"github.com/proxysql/golib/log"
	"github.com/proxysql/orchestrator/go/config"
)

var (
	hookOnce    sync.Once
	defaultHook = NewHook(nil, 0, 0, "")
)

func InitHook() {
	hookOnce.Do(func() {
		client := NewClient(
			config.Config.ProxySQLAdminAddress,
			config.Config.ProxySQLAdminPort,
			config.Config.ProxySQLAdminUser,
			config.Config.ProxySQLAdminPassword,
			config.Config.ProxySQLAdminUseTLS,
		)
		hook := NewHook(
			client,
			config.Config.ProxySQLWriterHostgroup,
			config.Config.ProxySQLReaderHostgroup,
			config.Config.ProxySQLPreFailoverAction,
		)
		if hook.IsConfigured() {
			log.Infof("ProxySQL hooks enabled: admin=%s:%d writer_hg=%d reader_hg=%d",
				config.Config.ProxySQLAdminAddress,
				config.Config.ProxySQLAdminPort,
				config.Config.ProxySQLWriterHostgroup,
				config.Config.ProxySQLReaderHostgroup,
			)
		} else if config.Config.ProxySQLAdminAddress != "" && config.Config.ProxySQLWriterHostgroup == 0 {
			log.Warningf("ProxySQL: ProxySQLAdminAddress is set but ProxySQLWriterHostgroup is 0 (unconfigured). ProxySQL hooks will be inactive.")
		}
		defaultHook = hook
	})
}

func GetHook() *Hook {
	return defaultHook
}
