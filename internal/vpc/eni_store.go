package vpc

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net"

	cappernet "capper/internal/network"
)

func (s *Store) InsertENI(e ENI) error {
	sdc := 1
	if !e.SourceDestCheck {
		sdc = 0
	}
	del := 1
	if !e.DeleteOnTermination {
		del = 0
	}
	_, err := s.db.Exec(
		`INSERT INTO capvpc_enis (id, vpc_id, subnet_id, zone_id, instance_id, attachment_index, mac_address, source_dest_check, delete_on_termination, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.VPCID, e.SubnetID, e.ZoneID, e.InstanceID, e.AttachmentIndex, e.MACAddress, sdc, del, e.Status, e.CreatedAt,
	)
	return err
}

func (s *Store) GetENI(id string) (ENI, error) {
	var e ENI
	var sdc, del int
	err := s.db.QueryRow(
		`SELECT id, vpc_id, subnet_id, zone_id, instance_id, attachment_index, mac_address, source_dest_check, delete_on_termination, status, created_at FROM capvpc_enis WHERE id=?`, id,
	).Scan(&e.ID, &e.VPCID, &e.SubnetID, &e.ZoneID, &e.InstanceID, &e.AttachmentIndex, &e.MACAddress, &sdc, &del, &e.Status, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return e, fmt.Errorf("eni %q not found", id)
	}
	e.SourceDestCheck = sdc == 1
	e.DeleteOnTermination = del == 1
	ips, _ := s.ListENIPrivateIPs(e.ID)
	for _, ip := range ips {
		e.PrivateIPAddresses = append(e.PrivateIPAddresses, ip)
		if e.PrimaryPrivateIP == "" {
			e.PrimaryPrivateIP = ip
		}
	}
	return e, err
}

func (s *Store) ListENIs(vpcID string) ([]ENI, error) {
	q := `SELECT id FROM capvpc_enis`
	var args []any
	if vpcID != "" {
		q += ` WHERE vpc_id=?`
		args = append(args, vpcID)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ENI
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		e, err := s.GetENI(id)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) UpdateENIAttachment(id, instanceID string, index int, status string) error {
	_, err := s.db.Exec(`UPDATE capvpc_enis SET instance_id=?, attachment_index=?, status=? WHERE id=?`, instanceID, index, status, id)
	return err
}

func (s *Store) DeleteENI(id string) error {
	_, err := s.db.Exec(`DELETE FROM capvpc_eni_private_ips WHERE eni_id=?`, id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM capvpc_enis WHERE id=?`, id)
	return err
}

func (s *Store) InsertENIPrivateIP(eniID, address string, primary bool) error {
	p := 0
	if primary {
		p = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO capvpc_eni_private_ips (id, eni_id, address, is_primary, status) VALUES (?, ?, ?, ?, 'assigned')`,
		newID("pip"), eniID, address, p,
	)
	return err
}

func (s *Store) ListENIPrivateIPs(eniID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT address FROM capvpc_eni_private_ips WHERE eni_id=? ORDER BY is_primary DESC`, eniID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		out = append(out, ip)
	}
	return out, rows.Err()
}

// AllocateSubnetIP picks the next free host address in a subnet CIDR.
func AllocateSubnetIP(subnetCIDR, vpcCIDR string, used []string) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return "", err
	}
	usedSet := map[string]bool{}
	for _, u := range used {
		usedSet[u] = true
	}
	gateway, _ := cappernet.GatewayForSubnet(subnetCIDR)
	if gateway != "" {
		usedSet[gateway] = true
	}
	ip := ipNet.IP.Mask(ipNet.Mask)
	for i := 0; i < 1<<16; i++ {
		incIP(ip)
		s := ip.String()
		if !ipNet.Contains(ip) {
			break
		}
		if usedSet[s] {
			continue
		}
		// skip network and broadcast-ish last octet for /24+
		if s == ipNet.IP.String() {
			continue
		}
		return s, nil
	}
	return "", fmt.Errorf("no available IPs in subnet %s", subnetCIDR)
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func randomMAC() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	b[0] &^= 1 // unicast
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}
