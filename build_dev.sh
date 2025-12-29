# Build with dev tag, and add to local path.
echo "building and installing gin binary with go install ..."
go install -tags=dev .
echo "success! you can now use the gim program"
rm gim
