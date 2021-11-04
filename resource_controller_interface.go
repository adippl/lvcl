/*
 *  lvcl is a simple program clustering libvirt servers
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

const(
	resource_controller_id_libvirt=iota
	resource_controller_id_docker
	
	resource_state_starting
	resource_state_running
	resource_state_stopping
	resource_state_stopped
	resource_state_paused
	resource_state_other
	
	
	utilization_hw_cores
	utilization_hw_threads
	utilization_hw_numaNodes
	utilization_hw_core_pre_numa
	utilization_mem_total_kb
	utilization_mem_free_kb
	utilization_mem_cached_kb
	utilization_mem_avalible_kb
	utilization_mem_buffers_kb
	utilization_swap_total_kb
	utilization_swap_free_kb
	utilization_swap_cached_kb
	utilization_load_1_100		//load average 1 minutes * 100
	utilization_load_5_100		//load average 5 minutes * 100
	utilization_load_15_100		//load average 15 minutes * 100
	utilization_cpu_time_kernel
	utilization_cpu_time_user
	utilization_cpu_time_io
	utilization_cpu_time_idle
	)

// generalized HARDWARE resource
// could be number of cores, RAM usage, cpu load etc
type utilization struct {
	resourceController_name	string
	resourceController_id	int
	name	string
	id		int
	value	uint64
}

// generalized cluster resource deployed by balance system
type cluster_resource struct {
	resourceController_name	string
	resourceController_id	int
	name		string
	id			int
	state		int
	resource	interface{}
}


type resourceController interface {
	get_running_resources() *[]cluster_resource
	get_utilization() *[]utilization
	start_resource(name string) bool
	stop_resource(name string) bool
	nuke_resource(name string) bool
	migrate_resource(resource_name string, dest_node string) bool
	clean_resource(name string) bool
}
