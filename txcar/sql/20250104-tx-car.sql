CREATE TABLE tx_car (
                        car_key uuid PRIMARY KEY NOT NULL,
                        version INT NOT NULL,
                        piece_cid TEXT NOT NULL,
                        payload_cid TEXT NOT NULL,
                        piece_size BIGINT NOT NULL, -- padded size
                        car_size BIGINT NOT NULL, -- raw size
                        create_time TIMESTAMP DEFAULT current_timestamp,
                        update_time TIMESTAMP DEFAULT current_timestamp,

                        constraint tx_car_piece_cid_key
                            unique (piece_cid),
                        constraint tx_car_payload_cid_key
                            unique (payload_cid)
);

CREATE INDEX tx_car_piece_cid_index
    on tx_car (piece_cid);