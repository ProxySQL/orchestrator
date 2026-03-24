package proxysql

import (
	"fmt"

	"github.com/proxysql/golib/log"
)

type Hook struct {
	client            *Client
	writerHostgroup   int
	readerHostgroup   int
	preFailoverAction string
}

func NewHook(client *Client, writerHostgroup, readerHostgroup int, preFailoverAction string) *Hook {
	if preFailoverAction == "" {
		preFailoverAction = "offline_soft"
	}
	return &Hook{
		client:            client,
		writerHostgroup:   writerHostgroup,
		readerHostgroup:   readerHostgroup,
		preFailoverAction: preFailoverAction,
	}
}

func (h *Hook) IsConfigured() bool {
	return h.client != nil && h.writerHostgroup > 0
}

// GetClient returns the underlying ProxySQL client, or nil if the hook is not configured.
func (h *Hook) GetClient() *Client {
	if h == nil {
		return nil
	}
	return h.client
}

func (h *Hook) PreFailover(oldMasterHost string, oldMasterPort int) error {
	if !h.IsConfigured() {
		return nil
	}
	query, args := buildPreFailoverSQL(h.preFailoverAction, oldMasterHost, oldMasterPort, h.writerHostgroup)
	if query == "" {
		log.Infof("proxysql: pre-failover: skipping drain of old master %s:%d (action=%s)", oldMasterHost, oldMasterPort, h.preFailoverAction)
		return nil
	}
	log.Infof("proxysql: pre-failover: draining old master %s:%d (action=%s)", oldMasterHost, oldMasterPort, h.preFailoverAction)
	if err := h.client.Exec(query, args...); err != nil {
		return fmt.Errorf("proxysql: pre-failover drain failed: %v", err)
	}
	if err := h.client.Exec("LOAD MYSQL SERVERS TO RUNTIME"); err != nil {
		return fmt.Errorf("proxysql: pre-failover LOAD TO RUNTIME failed: %v", err)
	}
	log.Infof("proxysql: pre-failover: drained old master %s:%d", oldMasterHost, oldMasterPort)
	return nil
}

func (h *Hook) PostFailover(newMasterHost string, newMasterPort int, oldMasterHost string, oldMasterPort int) error {
	if !h.IsConfigured() {
		return nil
	}
	log.Infof("proxysql: post-failover: promoting %s:%d as writer in hostgroup %d", newMasterHost, newMasterPort, h.writerHostgroup)
	sqls, sqlArgs := buildPostFailoverSQL(newMasterHost, newMasterPort, oldMasterHost, oldMasterPort, h.writerHostgroup, h.readerHostgroup)
	for i, query := range sqls {
		if err := h.client.Exec(query, sqlArgs[i]...); err != nil {
			return fmt.Errorf("proxysql: post-failover exec failed: %v", err)
		}
	}
	if err := h.client.Exec("LOAD MYSQL SERVERS TO RUNTIME"); err != nil {
		return fmt.Errorf("proxysql: post-failover LOAD TO RUNTIME failed: %v", err)
	}
	if err := h.client.Exec("SAVE MYSQL SERVERS TO DISK"); err != nil {
		log.Errorf("proxysql: post-failover SAVE TO DISK failed (non-fatal): %v", err)
	}
	log.Infof("proxysql: post-failover: promoted %s:%d as writer", newMasterHost, newMasterPort)
	return nil
}

func buildPreFailoverSQL(action, host string, port, writerHostgroup int) (string, []interface{}) {
	args := []interface{}{host, port, writerHostgroup}
	switch action {
	case "offline_soft":
		return "UPDATE mysql_servers SET status='OFFLINE_SOFT' WHERE hostname=? AND port=? AND hostgroup_id=?", args
	case "weight_zero":
		return "UPDATE mysql_servers SET weight=0 WHERE hostname=? AND port=? AND hostgroup_id=?", args
	case "none":
		return "", nil
	default:
		_ = log.Warningf("proxysql: unknown preFailoverAction '%s', defaulting to 'offline_soft'", action)
		return "UPDATE mysql_servers SET status='OFFLINE_SOFT' WHERE hostname=? AND port=? AND hostgroup_id=?", args
	}
}

func buildPostFailoverSQL(newHost string, newPort int, oldHost string, oldPort int, writerHostgroup, readerHostgroup int) ([]string, [][]interface{}) {
	sqls := []string{
		"DELETE FROM mysql_servers WHERE hostname=? AND port=? AND hostgroup_id=?",
		"REPLACE INTO mysql_servers (hostgroup_id, hostname, port) VALUES (?, ?, ?)",
	}
	args := [][]interface{}{
		{oldHost, oldPort, writerHostgroup},
		{writerHostgroup, newHost, newPort},
	}
	if readerHostgroup > 0 {
		sqls = append(sqls, "DELETE FROM mysql_servers WHERE hostname=? AND port=? AND hostgroup_id=?")
		args = append(args, []interface{}{newHost, newPort, readerHostgroup})
		sqls = append(sqls, "REPLACE INTO mysql_servers (hostgroup_id, hostname, port, status) VALUES (?, ?, ?, 'OFFLINE_SOFT')")
		args = append(args, []interface{}{readerHostgroup, oldHost, oldPort})
	}
	return sqls, args
}
