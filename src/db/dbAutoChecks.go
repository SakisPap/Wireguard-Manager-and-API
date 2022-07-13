package db

import (
	"errors"
	"log"
	"time"

	"gitlab.com/raspberry.tech/wireguard-manager-and-api/src/logger"
	"gitlab.com/raspberry.tech/wireguard-manager-and-api/src/manager"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

func AddRemovePeers() bool {
	getInterfaces, err := manager.GetInterfaces()
	if !logger.ErrorHandler("Info - Finding interfaces", err) {
		return false
	}

	for interfaces := 0; interfaces < len(getInterfaces); interfaces++ { //get interfaces
		for peer := 0; peer < len(getInterfaces[interfaces].Peers); peer++ {
			currentPeer := getInterfaces[interfaces].Peers[peer] //get the current peer in for loop
			interfaceName := getInterfaces[interfaces].Name
			updateBW := manager.AddRemovePeer(currentPeer, interfaceName)
			if updateBW {
				updatePeerBW(currentPeer)
			}
		}
	}
	return true
}

func BWPeerCheck() bool {
	getInterfaces, err := manager.GetInterfaces()
	if !logger.ErrorHandler("Info - Finding interfaces", err) {
		return false
	}

	db := DBSystem
	currentTime := time.Now().UTC()
	for interfaces := 0; interfaces < len(getInterfaces); interfaces++ { //get interfaces
		for peer := 0; peer < len(getInterfaces[interfaces].Peers); peer++ { //get each peer in the wg interface
			currentPeer := getInterfaces[interfaces].Peers[peer] //get the current peer in for loop

			publicKey := currentPeer.PublicKey     //get public key of client
			bwCurrent := currentPeer.TransmitBytes // bandwidth used
			pubKeyStr := publicKey.String()
			var subStruct Subscription

			resultIP := db.Where("public_key = ?", pubKeyStr).First(&subStruct) //find subscription record
			if errors.Is(resultIP.Error, gorm.ErrRecordNotFound) {
				log.Println("Could not find public key in database: ", pubKeyStr)
				continue
			}

			bwStoredUsage := subStruct.BandwidthUsed
			bwLimit := subStruct.BandwidthAllotted
			subEnd := subStruct.SubscriptionEnd

			subFormatted, subErr := time.Parse("2006-Jan-02 03:04:05 PM", subEnd)
			if !logger.ErrorHandler("Error - Parsing stored time ", subErr) {
				continue
			}
			if (bwStoredUsage+(bwCurrent/1000000) > bwLimit || currentTime.After(subFormatted)) && bwLimit != 0 {
				keyID := subStruct.KeyID
				updatePeerBW(currentPeer) //update bandwidth before disabling
				DisableKey(keyID)         //disable key if bandwidth limit reached or subscription end#
				log.Println("Info - Disabling key, bw or sub has ended, KeyID: ", keyID)
			}
		}
	}
	return true
}

func updatePeerBW(currentPeer wgtypes.Peer) {
	db := DBSystem
	var subStruct Subscription

	pubKey := currentPeer.PublicKey.String()
	currentBytes := currentPeer.TransmitBytes

	resultSub := db.Where("public_key = ?", pubKey).First(&subStruct) //find IP not in use
	if errors.Is(resultSub.Error, gorm.ErrRecordNotFound) {
		log.Println("Error - Subscription not found")
		return //continue even on error
	}
	updatedBW := subStruct.BandwidthUsed + (currentBytes / 1000000)

	db.Model(&Subscription{}).Where("public_key = ?", pubKey).Update("bandwidth_used", updatedBW)
}
