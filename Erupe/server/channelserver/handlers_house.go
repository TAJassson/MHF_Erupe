package channelserver

import (
	"erupe-ce/common/byteframe"
	"erupe-ce/network/mhfpacket"
	"go.uber.org/zap"
)

func handleMsgMhfUpdateInterior(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfUpdateInterior)
	_, err := s.server.db.Exec("UPDATE characters SET house=$1 WHERE id=$2", pkt.InteriorData, s.charID)
	if err != nil {
		panic(err)
	}
	doAckSimpleSucceed(s, pkt.AckHandle, make([]byte, 4))
}

func handleMsgMhfEnumerateHouse(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfEnumerateHouse)
	doAckSimpleSucceed(s, pkt.AckHandle, []byte{0x00, 0x00, 0x00, 0x00})
}

func handleMsgMhfUpdateHouse(s *Session, p mhfpacket.MHFPacket) {}

func handleMsgMhfLoadHouse(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfLoadHouse)
	bf := byteframe.NewByteFrame()
	var data []byte
	err := s.server.db.QueryRow("SELECT house FROM characters WHERE id=$1", s.charID).Scan(&data)
	if err != nil {
		panic(err)
	}
	if data == nil {
		data = make([]byte, 20)
	}
	if pkt.CharID != s.charID {
		bf.WriteBytes(make([]byte, 219))
	}
	bf.WriteBytes(data)
	doAckBufSucceed(s, pkt.AckHandle, bf.Data())
}

func handleMsgMhfGetMyhouseInfo(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfGetMyhouseInfo)

	var data []byte
	err := s.server.db.QueryRow("SELECT trophy FROM characters WHERE id = $1", s.charID).Scan(&data)
	if err != nil {
		panic(err)
	}
	if len(data) > 0 {
		doAckBufSucceed(s, pkt.AckHandle, data)
	} else {
		doAckBufSucceed(s, pkt.AckHandle, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	}
}

func handleMsgMhfUpdateMyhouseInfo(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfUpdateMyhouseInfo)

	_, err := s.server.db.Exec("UPDATE characters SET trophy=$1 WHERE id=$2", pkt.Unk0, s.charID)
	if err != nil {
		panic(err)
	}
	doAckSimpleSucceed(s, pkt.AckHandle, []byte{0x00, 0x00, 0x00, 0x00})
}

func handleMsgMhfLoadDecoMyset(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfLoadDecoMyset)
	var data []byte
	err := s.server.db.QueryRow("SELECT decomyset FROM characters WHERE id = $1", s.charID).Scan(&data)
	if err != nil {
		s.logger.Fatal("Failed to get preset decorations savedata from db", zap.Error(err))
	}

	if len(data) > 0 {
		doAckBufSucceed(s, pkt.AckHandle, data)
		//doAckBufSucceed(s, pkt.AckHandle, data)
	} else {
		// set first byte to 1 to avoid pop up every time without save
		body := make([]byte, 0x226)
		body[0] = 1
		doAckBufSucceed(s, pkt.AckHandle, body)
	}
}

func handleMsgMhfSaveDecoMyset(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfSaveDecoMyset)
	// https://gist.github.com/Andoryuuta/9c524da7285e4b5ca7e52e0fc1ca1daf
	var loadData []byte
	bf := byteframe.NewByteFrameFromBytes(pkt.RawDataPayload[1:]) // skip first unk byte
	err := s.server.db.QueryRow("SELECT decomyset FROM characters WHERE id = $1", s.charID).Scan(&loadData)
	if err != nil {
		s.logger.Fatal("Failed to get preset decorations savedata from db", zap.Error(err))
	} else {
		numSets := bf.ReadUint8() // sets being written
		// empty save
		if len(loadData) == 0 {
			loadData = []byte{0x01, 0x00}
		}

		savedSets := loadData[1] // existing saved sets
		// no sets, new slice with just first 2 bytes for appends later
		if savedSets == 0 {
			loadData = []byte{0x01, 0x00}
		}
		for i := 0; i < int(numSets); i++ {
			writeSet := bf.ReadUint16()
			dataChunk := bf.ReadBytes(76)
			setBytes := append([]byte{uint8(writeSet >> 8), uint8(writeSet & 0xff)}, dataChunk...)
			for x := 0; true; x++ {
				if x == int(savedSets) {
					// appending set
					if loadData[len(loadData)-1] == 0x10 {
						// sanity check for if there was a messy manual import
						loadData = append(loadData[:len(loadData)-2], setBytes...)
					} else {
						loadData = append(loadData, setBytes...)
					}
					savedSets++
					break
				}
				currentSet := loadData[3+(x*78)]
				if int(currentSet) == int(writeSet) {
					// replacing a set
					loadData = append(loadData[:2+(x*78)], append(setBytes, loadData[2+((x+1)*78):]...)...)
					break
				} else if int(currentSet) > int(writeSet) {
					// inserting before current set
					loadData = append(loadData[:2+((x)*78)], append(setBytes, loadData[2+((x)*78):]...)...)
					savedSets++
					break
				}
			}
			loadData[1] = savedSets // update set count
		}
		_, err := s.server.db.Exec("UPDATE characters SET decomyset=$1 WHERE id=$2", loadData, s.charID)
		if err != nil {
			s.logger.Fatal("Failed to update decomyset savedata in db", zap.Error(err))
		}
	}
	doAckSimpleSucceed(s, pkt.AckHandle, []byte{0x00, 0x00, 0x00, 0x00})
}

func handleMsgMhfEnumerateTitle(s *Session, p mhfpacket.MHFPacket) {
	pkt := p.(*mhfpacket.MsgMhfEnumerateTitle)
	bf := byteframe.NewByteFrame()
	titleCount := 114                  // all titles unlocked
	bf.WriteUint16(uint16(titleCount)) // title count
	bf.WriteUint16(0)                  // unk
	for i := 0; i < titleCount; i++ {
		bf.WriteUint16(uint16(i))
		bf.WriteUint16(0) // unk
		bf.WriteUint32(0) // timestamp acquired
		bf.WriteUint32(0) // timestamp updated
	}
	doAckBufSucceed(s, pkt.AckHandle, bf.Data())
}

func handleMsgMhfOperateWarehouse(s *Session, p mhfpacket.MHFPacket) {}

func handleMsgMhfEnumerateWarehouse(s *Session, p mhfpacket.MHFPacket) {}

func handleMsgMhfUpdateWarehouse(s *Session, p mhfpacket.MHFPacket) {}
