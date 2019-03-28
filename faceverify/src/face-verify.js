'use strict';

var machinebox = machinebox || {};

machinebox.FaceVerify = class {
	constructor(options) {
		this.options = options || {};
		this.options.facebox = this.options.facebox || "http://localhost:8080";
		this.options.snapshotInterval = this.options.snapshotInterval || 1000;
		this.options.onSecure = this.options.onSecure || function(){}
		this.options.onInsecure = this.options.onInsecure || function(){}
		this.options.error = this.options.error || function(error) {
			console.warn(error);
		}
		this.possible = true;
		this.canvas = document.createElement('canvas');
		this.video = document.querySelector(this.options.videoSelector);
		if (!this.video) {
			this.possible = false;
			this.options.error('face-verify: must provide a <video> via videoSelector option');
		}
		if (!this.hasGetUserMedia()) {
			this.possible = false;
			this.options.error('face-verify: getUserMedia is not supported in this browser');
		}
	}

	hasGetUserMedia() {
		return !!(navigator.getUserMedia || navigator.webkitGetUserMedia ||
			navigator.mozGetUserMedia || navigator.msGetUserMedia);
	}

	getUserMedia() {
		return navigator.getUserMedia || navigator.webkitGetUserMedia || 
			navigator.mozGetUserMedia || navigator.msGetUserMedia;
	}

	start() {
		if (!this.possible) {
			this.options.error('face-verify: cannot start (see previous errors)');
			return;
		}
		var handleError = function(e){
			this.options.error("failed to access webcam", e);
		}.bind(this)
		var mediaOptions = {
			video: {
				mandatory: {
					maxWidth: 400,
					maxHeight: 300,
				}
			}
		}

		navigator.mediaDevices.getUserMedia(mediaOptions).then(function(stream) {
			console.info(stream)
			this.video.srcObject = stream
			setTimeout(this.snapshot.bind(this), this.options.snapshotInterval);
		}.bind(this))
	}

	getBase64Snapshot() {
		this.canvas.width = this.video.videoWidth;
		this.canvas.height = this.video.videoHeight;
		var ctx = this.canvas.getContext('2d');
		ctx.drawImage(this.video, 0, 0, this.video.videoWidth, this.video.videoHeight);
		var dataURL = this.canvas.toDataURL("image/jpeg")
		return dataURL.slice("data:image/jpeg;base64,".length)
	}

	snapshot() {
		var imageData = this.getBase64Snapshot()
		var xhr = new XMLHttpRequest();
		xhr.open("POST", this.options.facebox + '/facebox/check');
		xhr.setRequestHeader("Content-Type", "application/json;charset=UTF-8");
		xhr.onreadystatechange = function(){
			if (xhr.readyState != 4) { return }
			if (xhr.status != 200) {
				console.warn(xhr, arguments)
				var msg = 'bad response from Facebox'
				if (xhr.responseText) { 
					msg += ": " + xhr.responseText 
				} else {
					msg += ": Check the console for technical information"
				}
				this.options.error(msg)
				setTimeout(this.snapshot.bind(this), this.options.snapshotInterval);
				return
			}
			var repsonseText = xhr.responseText
			var response = JSON.parse(repsonseText)
			if (!response.success) {
				console.warn(xhr, arguments)
				this.options.error(response.error || "Facebox: something went wrong, check the console for technical information")
				setTimeout(this.snapshot.bind(this), this.options.snapshotInterval);
				return
			}
			this.options.error(null)
			if (response.facesCount == 0) {
				this.options.onInsecure("no faces detected")
			} else if (response.facesCount > 1) {
				this.options.onInsecure("multuple faces detected")
			} else {
				if (!response.faces[0].matched) {
					this.options.onInsecure("Facebox doesn't recognize you")
				} else {
					this.options.onSecure(response.faces[0].name)
				}
			}
			setTimeout(this.snapshot.bind(this), this.options.snapshotInterval);
		}.bind(this)
		xhr.send(JSON.stringify({ base64: imageData }));
	}

}
