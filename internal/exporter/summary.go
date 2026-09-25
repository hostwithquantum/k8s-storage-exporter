package exporter

// summary mirrors the parts of the kubelet stats/summary response we need.
type summary struct {
	Pods []podStats `json:"pods"`
}

type podStats struct {
	PodRef     podRef           `json:"podRef"`
	Volumes    []volumeStats    `json:"volume"`
	Containers []containerStats `json:"containers"`
}

type podRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type fsStats struct {
	AvailableBytes *uint64 `json:"availableBytes"`
	CapacityBytes  *uint64 `json:"capacityBytes"`
	UsedBytes      *uint64 `json:"usedBytes"`
	Inodes         *uint64 `json:"inodes"`
	InodesFree     *uint64 `json:"inodesFree"`
	InodesUsed     *uint64 `json:"inodesUsed"`
}

type volumeStats struct {
	fsStats
	Name string `json:"name"`
}

// containerStats mirrors the per-container part of the kubelet summary.
// Rootfs is the container's writable layer, Logs its log directory; both
// count toward the container's ephemeral-storage limit.
type containerStats struct {
	Name   string   `json:"name"`
	Rootfs *fsStats `json:"rootfs"`
	Logs   *fsStats `json:"logs"`
}
