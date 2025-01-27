package channelserver

import (
	"sync"

	"time"

	"erupe-ce/common/byteframe"
	"erupe-ce/network/mhfpacket"
)

// StageObject holds infomation about a specific stage object.
type StageObject struct {
	sync.RWMutex
	id          uint32
	ownerCharID uint32
	x, y, z     float32
}

type ObjectMap struct {
	id     uint8
	charid uint32
	status bool
}

// stageBinaryKey is a struct used as a map key for identifying a stage binary part.
type stageBinaryKey struct {
	id0 uint8
	id1 uint8
}

// Stage holds stage-specific information
type Stage struct {
	sync.RWMutex

	// Stage ID string
	id string

	// Total count of objects ever created for this stage. Used for ObjID generation.
	gameObjectCount uint32

	// Save all object in stage
	objects map[uint32]*StageObject

	objectList map[uint8]*ObjectMap
	// Map of session -> charID.
	// These are clients that are CURRENTLY in the stage
	clients map[*Session]uint32

	// Map of charID -> bool, key represents whether they are ready
	// These are clients that aren't in the stage, but have reserved a slot (for quests, etc).
	reservedClientSlots map[uint32]bool

	// These are raw binary blobs that the stage owner sets,
	// other clients expect the server to echo them back in the exact same format.
	rawBinaryData map[stageBinaryKey][]byte

	maxPlayers uint16
	password   string
	createdAt  string
}

// NewStage creates a new stage with intialized values.
func NewStage(ID string) *Stage {
	s := &Stage{
		id:                  ID,
		clients:             make(map[*Session]uint32),
		reservedClientSlots: make(map[uint32]bool),
		objects:             make(map[uint32]*StageObject),
		rawBinaryData:       make(map[stageBinaryKey][]byte),
		maxPlayers:          4,
		gameObjectCount:     1,
		objectList:          make(map[uint8]*ObjectMap),
		createdAt:           time.Now().Format("01-02-2006 15:04:05"),
	}
	s.InitObjectList()
	return s
}

// BroadcastMHF queues a MHFPacket to be sent to all sessions in the stage.
func (s *Stage) BroadcastMHF(pkt mhfpacket.MHFPacket, ignoredSession *Session) {
	// Broadcast the data.
	for session := range s.clients {
		if session == ignoredSession {
			continue
		}

		// Make the header
		bf := byteframe.NewByteFrame()
		bf.WriteUint16(uint16(pkt.Opcode()))

		// Build the packet onto the byteframe.
		pkt.Build(bf, session.clientContext)

		// Enqueue in a non-blocking way that drops the packet if the connections send buffer channel is full.
		session.QueueSendNonBlocking(bf.Data())
	}
}

func (s *Stage) InitObjectList() {
	for seq := uint8(0x7f); seq > uint8(0); seq-- {
		newObj := &ObjectMap{
			id:     seq,
			charid: uint32(0),
			status: false,
		}
		s.objectList[seq] = newObj
	}
}

func (s *Stage) isCharInQuestByID(charID uint32) bool {
	if _, exists := s.reservedClientSlots[charID]; exists {
		return exists
	}

	return false
}

func (s *Stage) isQuest() bool {
	return len(s.reservedClientSlots) > 0
}

func (stage *Stage) GetName() string {
	switch stage.id {
	case MezeportaStageId:
		return "Mezeporta"
	case GuildHallLv1StageId:
		return "Guild Hall Lv1"
	case GuildHallLv2StageId:
		return "Guild Hall Lv2"
	case GuildHallLv3StageId:
		return "Guild Hall Lv3"
	case PugiFarmStageId:
		return "Pugi Farm"
	case RastaBarStageId:
		return "Rasta Bar"
	case PalloneCaravanStageId:
		return "Pallone Caravan"
	case GookFarmStageId:
		return "Gook Farm"
	case DivaFountainStageId:
		return "Diva Fountain"
	case DivaHallStageId:
		return "Diva Hall"
	case MezFesStageId:
		return "Mez Fes"
	default:
		return ""
	}
}

func (s *Stage) GetNewObjectID(CharID uint32) uint32 {
	ObjId := uint8(0)
	for seq := uint8(0x7f); seq > uint8(0); seq-- {
		if s.objectList[seq].status == false {
			ObjId = seq
			break
		}
	}
	s.objectList[ObjId].status = true
	s.objectList[ObjId].charid = CharID
	bf := byteframe.NewByteFrame()
	bf.WriteUint8(uint8(0))
	bf.WriteUint8(ObjId)
	bf.WriteUint16(uint16(0))
	obj := uint32(bf.Data()[3]) | uint32(bf.Data()[2])<<8 | uint32(bf.Data()[1])<<16 | uint32(bf.Data()[0])<<32
	return obj
}
