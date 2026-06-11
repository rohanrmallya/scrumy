
docker buildx build --platform linux/amd64 -t rohanmallya/scrumy:latest --load .

TODAYS_DATE=$(date +"%Y-%m-%d-%H%M")
echo $TODAYS_DATE
docker tag rohanmallya/scrumy:latest rohanmallya/scrumy:$TODAYS_DATE
docker push rohanmallya/scrumy:$TODAYS_DATE
docker push rohanmallya/scrumy:latest