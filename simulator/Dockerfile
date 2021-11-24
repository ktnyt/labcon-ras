FROM golang:1.17.3-buster as build

WORKDIR /app
COPY . /app
RUN go get -u -d . && go build -o ras .

FROM golang:1.17.3-buster

WORKDIR /app
COPY --from=build /app/ras /app/ras
RUN curl -LO https://raw.githubusercontent.com/vishnubob/wait-for-it/master/wait-for-it.sh && chmod +x /app/wait-for-it.sh

CMD [ "/app/wait-for-it.sh", "operator:5000", "--", "/app/ras" ]
