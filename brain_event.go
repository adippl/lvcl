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
import "strconv"

type event struct {
	ID				uint64
	EvName			string
	ActionID		int
	ResCtlID		int
	CreationTime	time.Time
	TimeoutTime		time.Time
	Actor			string
	Subject			string
	ResName			string
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

func (b *Brain)getEventById(id uint64) *event {
	var e *event = nil
	b.rwmux_events.RLock()
	for k,_:=range b.clusterEvents {
		if b.clusterEvents[k].ID == id {
			e = &b.clusterEvents[k]}}
	b.rwmux_events.RUnlock()
	return e}


func (b *Brain)getEventByEvName(EvName *string) *event {
	var e *event = nil
	b.rwmux_events.RLock()
	for k,_:=range b.clusterEvents {
		if b.clusterEvents[k].EvName == *EvName {
			e = &b.clusterEvents[k]}}
	b.rwmux_events.RUnlock()
	return e}

func (b *Brain)getEventByRes(r *Cluster_resource) *event {
	var e *event = nil
	b.rwmux_events.RLock()
	for k,_:=range b.clusterEvents {
		if b.clusterEvents[k].ResCtlID == r.ResourceController_id && b.clusterEvents[k].ResName == r.Name {
			e = &b.clusterEvents[k]}}
	b.rwmux_events.RUnlock()
	return e}


func (b *Brain)append_clusterEvents( e *event ){
	lg.msg_debug(5, fmt.Sprintf("append_clusterEvents() adds event %d %s to clusterEvents",
		e.ID,
		e.EvName))
	b.rwmux_events.Lock()
	b.clusterEvents = append( b.clusterEvents , *e)
//	b.eventEpoch++ 
	b.rwmux_events.Unlock()
	b.send_events_to_nodes(e)
	}


func (b *Brain)send_events_to_nodes( e *event){
	var m *message = brainNewMessage()
	m.DestHost="__everyone__"
	m.RpcFunc=brainNotifyAboutNewClusterEvent
	m.Argv = []string{
		strconv.FormatUint(e.ID,10),
		e.EvName,
		strconv.Itoa(e.ActionID),
		strconv.Itoa(e.ResCtlID),
		e.CreationTime.Format(time.RFC3339),
		e.TimeoutTime.Format(time.RFC3339),
		e.Actor,
		e.Subject,
		e.ResName,
		}
	m.Custom1 = *e

	//lg.msg_debug(5, fmt.Sprintf("Brain.send_events_to_nodes() %T %+v %T %+v", *e, *e, m.Custom1, m.Custom1))
	lg.msg_debug(5, fmt.Sprintf("Brain.send_events_to_nodes() %s", e.EvName))
	b.brn_ex <- *m}


func (b *Brain)msg_handle_brainNotifyAboutNewClusterEvent(m *message) bool {
	var u uint64 = 0
	var i1 int = 0
	var i2 int = 0
	var e error = nil
	if m.RpcFunc == brainNotifyAboutNewClusterEvent {
		

		event := event{
			EvName: 		m.Argv[1],
			Actor:			m.Argv[6],
			Subject:		m.Argv[7],
			ResName:		m.Argv[8],
		}
		u,e = strconv.ParseUint(m.Argv[0], 10, 64)
		if e != nil {
			// network critical function, logging function has to run as separate thread to prevent lockup
			go lg.err("Brain.msg_handle_brainNotifyAboutNewClusterEvent() u err", e)
			return true}
		e=nil
		event.ID = u
		
		i1,e = strconv.Atoi(m.Argv[2])
		if e != nil {
			// network critical function, logging function has to run as separate thread to prevent lockup
			go lg.err("Brain.msg_handle_brainNotifyAboutNewClusterEvent() i1 err", e)
			return true}
		e=nil
		event.ActionID = i1

		i2,e = strconv.Atoi(m.Argv[3])
		if e != nil {
			// network critical function, logging function has to run as separate thread to prevent lockup
			go lg.err("Brain.msg_handle_brainNotifyAboutNewClusterEvent() i2 err", e)
			return true}
		e=nil
		event.ResCtlID = i2
		
		t1,e := time.Parse(time.RFC3339, m.Argv[4])
		if e != nil {
			// network critical function, logging function has to run as separate thread to prevent lockup
			go lg.err("Brain.msg_handle_brainNotifyAboutNewClusterEvent() t1 err", e)
			return true}
		e=nil
		event.CreationTime = t1

		t2,e := time.Parse(time.RFC3339, m.Argv[5])
		if e != nil {
			// network critical function, logging function has to run as separate thread to prevent lockup
			go lg.err("Brain.msg_handle_brainNotifyAboutNewClusterEvent() t2 err", e)
			return true}
		event.TimeoutTime = t2

		if b.getEventById( event.ID ) == nil{
			b.append_clusterEvents( &event )
			// network critical function, logging function has to run as separate thread to prevent lockup
			go lg.msg_debug(5, fmt.Sprintf("Brain.msg_handle_brainNotifyAboutNewClusterEvent() adds event %d %s to clusterEvents",
				event.ID,
				event.EvName))}
		return true}
	return false}



//func (b *Brain)event_Sender(){
//	b.rwmux_ec.RLock()
//	if b.eventEpoch > b.eventEpoch_old {
//		lg.msg_debug(4, "b.eventEpoch changed. sending events to other nodes")
//		
//	b.rwmux_ec.RUnlock()
//	}


//func (b *Brain)zero_eventEpoch(){
//	lg.msg_debug(5, "zero_eventEpoch() runs")
//	b.rwmux_events.Lock()
//	b.eventEpoch = 0;
//	b.eventEpoch_old = 0;
//	b.rwmux_events.Unlock()
//	}

func (e1 *event)compare_events_timeout_check_e2(e2 *event) bool {
	if e1.ActionID == e2.ActionID &&
		e1.ResCtlID == e2.ResCtlID &&
		e1.ResName == e2.ResName &&
		e1.EvName == e2.EvName &&
		e1.checktimeout() != true {
		
		return true}
	return false}

func (b *Brain)check_if_event_already_exist_and_active( e *event ) bool {
	b.rwmux_events.RLock()
	for k,_ := range b.clusterEvents {
		if e.compare_events_timeout_check_e2( &b.clusterEvents[k] ) {
			b.rwmux_events.RUnlock()
			return true}}
	b.rwmux_events.RUnlock()
	return false}


func (b *Brain)create_event_start_resource(r *Cluster_resource) bool {
	e := createEvent()
	e.ID = rand.Uint64()
	e.EvName = fmt.Sprintf("starting %s on %s with %s", r.Name, r.Placement, r.CtlString())
	e.ActionID = resource_state_running
	e.ResCtlID = r.ResourceController_id
	e.ResName = r.Name
	e.Actor = r.Placement
	
	// check if exists event with the same ID. Recreate ID and check again
	for b.getEventById( e.ID ) != nil {
		e.ID = rand.Uint64()}
	
	if b.check_if_event_already_exist_and_active(e) {
		lg.msg_debug(5, fmt.Sprintf(
			"create_event_start_resource() found that event '%s' already exist on the cluster. Not adding",
			e.EvName))
		return false}
	b.append_clusterEvents(e)
	return true}

func (b *Brain)create_event_stop_resource(r *Cluster_resource) bool {
	e := createEvent()
	e.ID = rand.Uint64()
	e.EvName = fmt.Sprintf("stopping %s on %s with %s", r.Name, r.Placement, r.CtlString())
	e.ActionID = resource_state_stopped
	e.ResCtlID = r.ResourceController_id
	e.ResName = r.Name
	e.Actor = r.Placement
	
	// check if exists event with the same ID. Recreate ID and check again
	for b.getEventById( e.ID ) != nil {
		e.ID = rand.Uint64()}
	
	if b.check_if_event_already_exist_and_active(e) {
		lg.msg_debug(5, fmt.Sprintf(
			"create_event_stop_resource() found that event '%s' already exist on the cluster. Not adding",
			e.EvName))
		return false}
	b.append_clusterEvents(e)
	return true}


func (b *Brain)create_event_nuke_resource(r *Cluster_resource) bool {
	e := createEvent()
	e.ID = rand.Uint64()
	e.EvName = fmt.Sprintf("nuking %s on %s with %s", r.Name, r.Placement, r.CtlString())
	e.ActionID = resource_state_nuked
	e.ResCtlID = r.ResourceController_id
	e.Actor = r.Placement
	
	// check if exists event with the same ID. Recreate ID and check again
	for b.getEventById( e.ID ) != nil {
		e.ID = rand.Uint64()}
	
	if b.check_if_event_already_exist_and_active(e) {
		lg.msg_debug(5, fmt.Sprintf(
			"create_event_start_resource() found that event '%s' already exist on the cluster. Not adding",
			e.EvName))
		return false}
	b.append_clusterEvents(e)
	return true}
