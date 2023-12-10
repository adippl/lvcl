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

import "fmt"
import "time"
import "math/rand"

type event struct {
	ID				uint32
	Name			string
	ActionID		int
	ResCtlID		int
	CreationTime	time.Time
	TimeoutTime		time.Time
	Actor			string
	Subject			string
	Custom1			interface{}
	}

func createEvent() *event {
	var e event
	e.CreationTime = time.Now()
	e.TimeoutTime = e.CreationTime.Add(
		time.Millisecond * time.Duration( config.DefaultEventTimeoutTimeSec * 1000))
	return &e }

func (e *event)checktimeout() bool {
	lg.msg_debug(5, fmt.Sprintf("checktimeout '%s' '%s' %b",
		time.Now().String(),
		e.TimeoutTime.String(),
		time.Now().After(e.TimeoutTime),
		))

	return time.Now().After(e.TimeoutTime) }


func (b *Brain)getEventById(id uint32) *event {
	var e *event = nil
	b.rwmux_events.RLock()
	for k,_:=range b.clusterEvents {
		if b.clusterEvents[k].ID == id {
			e = &b.clusterEvents[k]}}
	b.rwmux_events.RUnlock()
	return e}

func (b *Brain)append_clusterEvents( e *event ){
	b.rwmux_events.Lock()
	b.clusterEvents = append( b.clusterEvents , *e)
	b.rwmux_events.Unlock()
	}

func (e1 *event)compare_events_timeout_check_e2(e2 *event) bool {
	if e1.ActionID == e2.ActionID &&
		e1.ResCtlID == e2.ResCtlID &&
		e1.Name == e2.Name &&
		e2.checktimeout() != true {
		
		return true}
	return false}

func (b *Brain)check_if_event_already_exist_and_active( e *event ) bool {
	b.rwmux_events.Lock()
	for k,_ := range b.clusterEvents {
		if e.compare_events_timeout_check_e2( &b.clusterEvents[k] ) {
			b.rwmux_events.Unlock()
			return true}}
	b.clusterEvents = append( b.clusterEvents , *e)
	b.rwmux_events.Unlock()
	return false}

func (b *Brain)create_event_start_resource(r *Cluster_resource) bool {
	e := createEvent()
	e.ID = rand.Uint32()
	e.Name = fmt.Sprintf("starting %s on %s with %s", r.Name, r.Placement, r.CtlString())
	e.ActionID = resource_state_running
	e.ResCtlID = r.ResourceController_id
	
	// check if exists event with the same ID. Recreate ID and check again
	for b.getEventById( e.ID ) != nil {
		e.ID = rand.Uint32()}
	
	if b.check_if_event_already_exist_and_active(e) {
		lg.msg_debug(5, fmt.Sprintf(
			"create_event_start_resource() found that event '%s' already exist on the cluster. Not adding",
			e.Name))
	return false}
	b.append_clusterEvents(e)
	return true}

func (b *Brain)create_event_stop_resource(r *Cluster_resource) bool {
	e := createEvent()
	e.ID = rand.Uint32()
	e.Name = fmt.Sprintf("stopping %s on %s with %s", r.Name, r.Placement, r.CtlString())
	e.ActionID = resource_state_stopped
	e.ResCtlID = r.ResourceController_id
	
	// check if exists event with the same ID. Recreate ID and check again
	for b.getEventById( e.ID ) != nil {
		e.ID = rand.Uint32()}
	
	if b.check_if_event_already_exist_and_active(e) {
		lg.msg_debug(5, fmt.Sprintf(
			"create_event_start_resource() found that event '%s' already exist on the cluster. Not adding",
			e.Name))
	return false}
	b.append_clusterEvents(e)
	return true}


func (b *Brain)create_event_nuke_resource(r *Cluster_resource) bool {
	e := createEvent()
	e.ID = rand.Uint32()
	e.Name = fmt.Sprintf("nuking %s on %s with %s", r.Name, r.Placement, r.CtlString())
	e.ActionID = resource_state_nuked
	e.ResCtlID = r.ResourceController_id
	
	// check if exists event with the same ID. Recreate ID and check again
	for b.getEventById( e.ID ) != nil {
		e.ID = rand.Uint32()}
	
	if b.check_if_event_already_exist_and_active(e) {
		lg.msg_debug(5, fmt.Sprintf(
			"create_event_start_resource() found that event '%s' already exist on the cluster. Not adding",
			e.Name))
	return false}
	b.append_clusterEvents(e)
	return true}
