package proxysql

import (
	"fmt"

	"github.com/proxysql/golib/log"
)

// ServerEntry represents a row from ProxySQL's mysql_servers table.
type ServerEntry struct {
	HostgroupID int    `json:"hostgroup_id"`
	Hostname    string `json:"hostname"`
	Port        int    `json:"port"`
	Status      string `json:"status"`
	Weight      int    `json:"weight"`
	MaxConns    int    `json:"max_connections"`
	Comment     string `json:"comment"`
}

// GetServers queries runtime_mysql_servers from ProxySQL and returns all server entries.
// Returns an empty slice (no error) if the client is nil.
func (c *Client) GetServers() ([]ServerEntry, error) {
	if c == nil {
		return []ServerEntry{}, nil
	}

	query := "SELECT hostgroup_id, hostname, port, status, weight, max_connections, comment FROM runtime_mysql_servers ORDER BY hostgroup_id, hostname, port"
	rows, db, err := c.Query(query)
	if err != nil {
		return nil, fmt.Errorf("proxysql: GetServers: %v", err)
	}
	defer func() { _ = db.Close() }()
	defer func() { _ = rows.Close() }()

	var servers []ServerEntry
	for rows.Next() {
		var s ServerEntry
		if err := rows.Scan(&s.HostgroupID, &s.Hostname, &s.Port, &s.Status, &s.Weight, &s.MaxConns, &s.Comment); err != nil {
			return nil, fmt.Errorf("proxysql: GetServers scan: %v", err)
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("proxysql: GetServers rows: %v", err)
	}

	log.Infof("proxysql: GetServers returned %d entries", len(servers))
	return servers, nil
}

// GetServersByHostgroup returns servers filtered by hostgroup ID.
// Returns an empty slice (no error) if the client is nil.
func (c *Client) GetServersByHostgroup(hostgroupID int) ([]ServerEntry, error) {
	if c == nil {
		return []ServerEntry{}, nil
	}

	query := "SELECT hostgroup_id, hostname, port, status, weight, max_connections, comment FROM runtime_mysql_servers WHERE hostgroup_id = ? ORDER BY hostname, port"
	rows, db, err := c.Query(query, hostgroupID)
	if err != nil {
		return nil, fmt.Errorf("proxysql: GetServersByHostgroup(%d): %v", hostgroupID, err)
	}
	defer func() { _ = db.Close() }()
	defer func() { _ = rows.Close() }()

	var servers []ServerEntry
	for rows.Next() {
		var s ServerEntry
		if err := rows.Scan(&s.HostgroupID, &s.Hostname, &s.Port, &s.Status, &s.Weight, &s.MaxConns, &s.Comment); err != nil {
			return nil, fmt.Errorf("proxysql: GetServersByHostgroup scan: %v", err)
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("proxysql: GetServersByHostgroup rows: %v", err)
	}

	log.Infof("proxysql: GetServersByHostgroup(%d) returned %d entries", hostgroupID, len(servers))
	return servers, nil
}
