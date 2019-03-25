/**
 * Copyright (c) 2017 eBay Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 **/

package goovn

import (
	"strings"

	"github.com/ebay/libovsdb"
)

type LoadBalancer struct {
	UUID       string
	Name       string
	vips       map[interface{}]interface{}
	protocol   string
	ExternalID map[interface{}]interface{}
}

func (odbi *ovnDBImp) lbUpdateImp(name string, vipPort string, protocol string, addrs []string) (*OvnCommand, error) {
	row := make(OVNRow)

	// prepare vips map
	vipMap := make(map[string]string)
	vipMap[vipPort] = strings.Join(addrs, ",")

	oMap, err := libovsdb.NewOvsMap(vipMap)
	if err != nil {
		return nil, err
	}

	row["vips"] = oMap
	row["protocol"] = protocol

	condition := libovsdb.NewCondition("name", "==", name)

	insertOp := libovsdb.Operation{
		Op:    opUpdate,
		Table: tableLoadBalancer,
		Row:   row,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{insertOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lbAddImp(name string, vipPort string, protocol string, addrs []string) (*OvnCommand, error) {
	namedUUID, err := newRowUUID()
	if err != nil {
		return nil, err
	}

	row := make(OVNRow)
	row["name"] = name

	if uuid := odbi.getRowUUID(tableLoadBalancer, row); len(uuid) > 0 {
		return nil, ErrorExist
	}

	// prepare vips map
	vipMap := make(map[string]string)
	vipMap[vipPort] = strings.Join(addrs, ",")

	oMap, err := libovsdb.NewOvsMap(vipMap)
	if err != nil {
		return nil, err
	}
	row["vips"] = oMap
	row["protocol"] = protocol

	insertOp := libovsdb.Operation{
		Op:       opInsert,
		Table:    tableLoadBalancer,
		Row:      row,
		UUIDName: namedUUID,
	}

	mutateUUID := []libovsdb.UUID{{namedUUID}}
	mutateSet, err := libovsdb.NewOvsSet(mutateUUID)
	if err != nil {
		return nil, err
	}

	mutation := libovsdb.NewMutation("load_balancer", opInsert, mutateSet)
	// TODO: Add filter for LS name
	condition := libovsdb.NewCondition("name", "!=", "")

	mutateOp := libovsdb.Operation{
		Op:        opMutate,
		Table:     tableLogicalSwitch,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	operations := []libovsdb.Operation{insertOp, mutateOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) lbDelImp(name string) (*OvnCommand, error) {
	condition := libovsdb.NewCondition("name", "==", name)
	deleteOp := libovsdb.Operation{
		Op:    opDelete,
		Table: tableLoadBalancer,
		Where: []interface{}{condition},
	}
	operations := []libovsdb.Operation{deleteOp}
	return &OvnCommand{operations, odbi, make([][]map[string]interface{}, len(operations))}, nil
}

func (odbi *ovnDBImp) GetLB(name string) ([]*LoadBalancer, error) {
	var listLB []*LoadBalancer

	odbi.cachemutex.RLock()
	defer odbi.cachemutex.RUnlock()

	cacheLoadBalancer, ok := odbi.cache[tableLoadBalancer]
	if !ok {
		return nil, ErrorSchema
	}

	for uuid, drows := range cacheLoadBalancer {
		if lbName, ok := drows.Fields["name"].(string); ok && lbName == name {
			lb, err := odbi.rowToLB(uuid)
			if err != nil {
				return nil, err
			}
			listLB = append(listLB, lb)
		}
	}
	return listLB, nil
}

func (odbi *ovnDBImp) rowToLB(uuid string) (*LoadBalancer, error) {
	cacheLoadBalancer, ok := odbi.cache[tableLoadBalancer][uuid]
	if !ok {
		return nil, ErrorSchema
	}

	lb := &LoadBalancer{
		UUID:       uuid,
		protocol:   cacheLoadBalancer.Fields["protocol"].(string),
		Name:       cacheLoadBalancer.Fields["name"].(string),
		vips:       cacheLoadBalancer.Fields["vips"].(libovsdb.OvsMap).GoMap,
		ExternalID: cacheLoadBalancer.Fields["external_ids"].(libovsdb.OvsMap).GoMap,
	}

	return lb, nil
}