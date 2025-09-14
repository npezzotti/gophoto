# gophoto

### Start a development database
```
docker run --name gophoto-db -d -p 5432:5432 -e POSTGRES_USER=gophoto -e POSTGRES_PASSWORD=password -e POSTGRES_DB=gophoto postgres
```
