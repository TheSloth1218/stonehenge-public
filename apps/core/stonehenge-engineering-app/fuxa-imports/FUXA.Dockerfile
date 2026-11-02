FROM frangoteam/fuxa:1.3.3

WORKDIR /usr/src/app/FUXA/server

RUN npm install --no-audit --no-fund \
    mcprotocol@0.1.2 \
    modbus-serial@8.0.19 \
    node-red@4.1.10
