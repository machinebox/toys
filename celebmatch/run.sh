#/bin/bash
echo "Running Facebox..."

docker run -d -p "8080:8080" -e "MB_KEY=" machinebox/facebox 
sleep 5

echo "Uploading state file..."
curl -X POST -F 'file=@IMDB_labeled.machinebox.facebox' "http://localhost:8080/facebox/state"

echo "Running celebmatch..."
go build -o celebmatch
./celebmatch
