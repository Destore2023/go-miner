// Code generated by "stringer -type=nodeEvent"; DO NOT EDIT.

package discover

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[invalidEvent-0]
	_ = x[pingPacket-1]
	_ = x[pongPacket-2]
	_ = x[findnodePacket-3]
	_ = x[neighborsPacket-4]
	_ = x[findnodeHashPacket-5]
	_ = x[topicRegisterPacket-6]
	_ = x[topicQueryPacket-7]
	_ = x[topicNodesPacket-8]
	_ = x[pongTimeout-265]
	_ = x[pingTimeout-266]
	_ = x[neighboursTimeout-267]
}

const (
	_nodeEvent_name_0 = "invalidEventpingPacketpongPacketfindnodePacketneighborsPacketfindnodeHashPackettopicRegisterPackettopicQueryPackettopicNodesPacket"
	_nodeEvent_name_1 = "pongTimeoutpingTimeoutneighboursTimeout"
)

var (
	_nodeEvent_index_0 = [...]uint8{0, 12, 22, 32, 46, 61, 79, 98, 114, 130}
	_nodeEvent_index_1 = [...]uint8{0, 11, 22, 39}
)

func (i nodeEvent) String() string {
	switch {
	case i <= 8:
		return _nodeEvent_name_0[_nodeEvent_index_0[i]:_nodeEvent_index_0[i+1]]
	case 265 <= i && i <= 267:
		i -= 265
		return _nodeEvent_name_1[_nodeEvent_index_1[i]:_nodeEvent_index_1[i+1]]
	default:
		return "nodeEvent(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
