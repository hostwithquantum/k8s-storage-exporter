package exporter

// summary mirrors the parts of the kubelet stats/summary response we need.
type summary struct {
	Pods []podStats `json:"pods"`
}

type podStats struct {
	PodRef           podRef        `json:"podRef"`
	EphemeralStorage *fsStats      `json:"ephemeral-storage"`
	Volumes          []volumeStats `json:"volume"`
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
