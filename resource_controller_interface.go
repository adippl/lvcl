package main

const(
	utilization_hw_cores=iota
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
	value	int
}

// generalized cluster resource deployed by balance system
type cluster_resource struct {
	resourceController_name	string
	resourceController_id	int
	name		string
	id			int
	resource	interface{}
}


type resourceController interface {
	get_running_resources() []string
	get_utilization() []string
	start_resource(name string) bool
	stop_resource(name string) bool
	migrate_resource(resource_name string, dest_node string) bool
	clean_resourceStare(name string) bool
}
