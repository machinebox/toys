#/bin/bash
echo "Running Facebox..."

docker run -d -p "8080:8080" -e "MB_KEY=c2phZ2FkQGNsb3VkaWFuLmNvbXxwcml2YXRl._bwwzJOl69pR2A0kpYCBadXnMvcwXcPOVuKEXQNdSEmAw9VUkyyl7X2RvGS00AmvKcS-guPvansk-p5yWFT3Kg" machinebox/facebox 
sleep 5

echo "Uploading state file..."
curl -X POST -F 'file=@IMDB_labeled.machinebox.facebox' "http://localhost:8080/facebox/state"

echo "Running celebmatch..."
go build -o celebmatch
./celebmatch
