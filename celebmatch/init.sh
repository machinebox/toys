#/bin/bash

if ! [ -x "$(command -v docker)" ]; then
	echo "Please install Docker from https://www.docker.com/community-edition"
	exit 1
fi

if ! [ -x "$(command -v go)" ]; then
	echo "Please install Go from https://golang.org/dl/"
	exit 1
fi

if ! [ -x "$(command -v go)" ]; then
	echo "Please install wget, or download the dataset manually"
	exit 1
fi

echo "Downloading dependencies..."
go get 

echo "Downloading dataset..."
wget https://storage.googleapis.com/machinebox_datasets/faces/imdb/IMDB_labeled.machinebox.facebox

echo "Downloading source images..."
wget https://storage.googleapis.com/machinebox_datasets/faces/imdb/imdb_crop.tar

echo "Extracting images..."
tar -xf imdb_crop.tar.tar -C /public/images

echo "Installing Facebox..."
docker pull machinebox/facebox:latest

echo ""
echo "OK awesome, you're ready to go..."
echo ""
