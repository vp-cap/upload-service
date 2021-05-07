echo $1
if [ "$1" = "bo" ]
then
    echo ${GIT_USER}
    echo ${GIT_PASS}
    docker build --build-arg GIT_USER=${GIT_USER} --build-arg GIT_PASS=${GIT_PASS} -t vp-cap/upload-service .
elif [ "$1" = "br" ]
then
    docker build --build-arg GIT_USER=${GIT_USER} --build-arg GIT_PASS=${GIT_PASS} -t vp-cap/upload-service .
    docker stop upload-service && docker rm upload-service
    docker run --network=common --name upload-service -p 50051:50051 vp-cap/upload-service
else
    docker stop upload-service && docker rm upload-service
    docker run --network=common --name upload-service -p 50051:50051 vp-cap/upload-service
fi
