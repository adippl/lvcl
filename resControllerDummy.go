/*  lvcl is a simple program clustering libvirt servers
 *  Copyright (C) 2020 Adam Prycki (email: adam.prycki@gmail.com)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License along
 *  with this program; if not, write to the Free Software Foundation, Inc.,
 *  51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
 */
package main

import "sync"

type Dummy_rctl struct {
	rwmux			sync.RWMutex
	resources		[]Cluster_resource
	utilization		[]Cluster_utilization
}

func (c Dummy_rctl)Get_running_resources() *[]Cluster_resource {
	return &c.resources }

func (c Dummy_rctl)Get_utilization() *[]Cluster_utilization {
	return &c.utilization }

func (c Dummy_rctl)Start_resource(name string) bool {
	lg.msg("Dummy resource "+name+" starting")
	res := config.GetCluster_resourcebyName(&name)
	if res == nil {
		lg.msg("Dummy resource "+name+" doesn't exist")
		return false}
	res.State = resource_state_running
	c.resources = append(c.resources, *res)
	return true}

func (c Dummy_rctl)Stop_resource(name string) bool {
	lg.msg("Dummy resource "+name+" stopping")
	for k,_:=range c.resources {
		if c.resources[k].Name == name {
			c.resources[k].State = resource_state_stopped
			lg.msg("Dummy resource "+name+" stopped")
			return true}}
	lg.msg("Dummy resource "+name+" couldn't stop, doesn't exist")
	return false}

func (c Dummy_rctl)Nuke_resource(name string) bool {
	lg.msg("Dummy resource "+name+" Nukked")
	for k,_:=range c.resources {
		if c.resources[k].Name == name {
			c.resources[k].State = resource_state_stopped
			lg.msg("Dummy resource "+name+" nuked")
			return true}}
	return true}

func (c Dummy_rctl)Migrate_resource(resource_name string,
	dest_node string) bool {
	lg.msg("Dummy resource "+resource_name+" migrating to "+dest_node)
	return true}

func (c Dummy_rctl)Clean_resource(name string) bool {
	lg.msg("Dummy resource "+name+" cleanned up")
	for k,_:=range c.resources {
		if c.resources[k].Name == name {
			c.resources[k].State = resource_state_stopped
			lg.msg("Dummy resource "+name+" cleanup")
			return true}}
	return false}

func (c Dummy_rctl)Kill_controller() bool {
	lg.msg("Dummy resource controller Killed")
	return true}


func NewDummy() *Dummy_rctl {
	return &Dummy_rctl{
		rwmux:			sync.RWMutex{},
		resources:		make([]Cluster_resource, 0),
		utilization:	make([]Cluster_utilization, 0)}}

func (c Dummy_rctl)Get_controller_health() bool {
	return true }


func (c Dummy_rctl)Get_live_migration_support() bool {
	return false}
