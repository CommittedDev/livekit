// Copyright 2023 LiveKit, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rtc

import (
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/livekit-server/pkg/sfu/buffer"
	dd "github.com/livekit/livekit-server/pkg/sfu/dependencydescriptor"
	"github.com/livekit/mediatransportutil/pkg/rtcconfig"
)

const (
	frameMarking        = "urn:ietf:params:rtp-hdrext:framemarking"
	repairedRTPStreamID = "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id"
)

type WebRTCConfig struct {
	rtcconfig.WebRTCConfig

	BufferFactory *buffer.Factory
	Receiver      ReceiverConfig
	Publisher     DirectionConfig
	Subscriber    DirectionConfig
}

type ReceiverConfig struct {
	PacketBufferSizeVideo int
	PacketBufferSizeAudio int
}

type RTPHeaderExtensionConfig struct {
	Audio []string
	Video []string
}

type RTCPFeedbackConfig struct {
	Audio []webrtc.RTCPFeedback
	Video []webrtc.RTCPFeedback
}

type DirectionConfig struct {
	RTPHeaderExtension RTPHeaderExtensionConfig
	RTCPFeedback       RTCPFeedbackConfig
	StrictACKs         bool
}

func NewWebRTCConfig(conf *config.Config) (*WebRTCConfig, error) {
	rtcConf := conf.RTC

	webRTCConfig, err := rtcconfig.NewWebRTCConfig(&rtcConf.RTCConfig, conf.Development)
	if err != nil {
		return nil, err
	}

	// we don't want to use active TCP on a server, clients should be dialing
	webRTCConfig.SettingEngine.DisableActiveTCP(true)

	if rtcConf.PacketBufferSize == 0 {
		rtcConf.PacketBufferSize = 500
	}
	if rtcConf.PacketBufferSizeVideo == 0 {
		rtcConf.PacketBufferSizeVideo = rtcConf.PacketBufferSize
	}
	if rtcConf.PacketBufferSizeAudio == 0 {
		rtcConf.PacketBufferSizeAudio = rtcConf.PacketBufferSize
	}

	// publisher configuration
	publisherConfig := DirectionConfig{
		StrictACKs: true, // publisher is dialed, and will always reply with ACK
		RTPHeaderExtension: RTPHeaderExtensionConfig{
			Audio: []string{
				sdp.SDESMidURI,
				sdp.SDESRTPStreamIDURI,
				sdp.AudioLevelURI,
			},
			Video: []string{
				sdp.SDESMidURI,
				sdp.SDESRTPStreamIDURI,
				sdp.TransportCCURI,
				frameMarking,
				dd.ExtensionURI,
				repairedRTPStreamID,
			},
		},
		RTCPFeedback: RTCPFeedbackConfig{
			Audio: []webrtc.RTCPFeedback{
				{Type: webrtc.TypeRTCPFBNACK},
			},
			Video: []webrtc.RTCPFeedback{
				{Type: webrtc.TypeRTCPFBTransportCC},
				{Type: webrtc.TypeRTCPFBCCM, Parameter: "fir"},
				{Type: webrtc.TypeRTCPFBNACK},
				{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
			},
		},
	}

	// subscriber configuration
	subscriberConfig := DirectionConfig{
		StrictACKs: conf.RTC.StrictACKs,
		RTPHeaderExtension: RTPHeaderExtensionConfig{
			Video: []string{dd.ExtensionURI},
		},
		RTCPFeedback: RTCPFeedbackConfig{
			Video: []webrtc.RTCPFeedback{
				{Type: webrtc.TypeRTCPFBCCM, Parameter: "fir"},
				{Type: webrtc.TypeRTCPFBNACK},
				{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
			},
		},
	}
	if rtcConf.CongestionControl.UseSendSideBWE {
		subscriberConfig.RTPHeaderExtension.Video = append(subscriberConfig.RTPHeaderExtension.Video, sdp.TransportCCURI)
		subscriberConfig.RTCPFeedback.Video = append(subscriberConfig.RTCPFeedback.Video, webrtc.RTCPFeedback{Type: webrtc.TypeRTCPFBTransportCC})
	} else {
		subscriberConfig.RTPHeaderExtension.Video = append(subscriberConfig.RTPHeaderExtension.Video, sdp.ABSSendTimeURI)
		subscriberConfig.RTCPFeedback.Video = append(subscriberConfig.RTCPFeedback.Video, webrtc.RTCPFeedback{Type: webrtc.TypeRTCPFBGoogREMB})
	}

	return &WebRTCConfig{
		WebRTCConfig: *webRTCConfig,
		Receiver: ReceiverConfig{
			PacketBufferSizeVideo: rtcConf.PacketBufferSizeVideo,
			PacketBufferSizeAudio: rtcConf.PacketBufferSizeAudio,
		},
		Publisher:  publisherConfig,
		Subscriber: subscriberConfig,
	}, nil
}

func (c *WebRTCConfig) SetBufferFactory(factory *buffer.Factory) {
	c.BufferFactory = factory
	c.SettingEngine.BufferFactory = factory.GetOrNew
}
