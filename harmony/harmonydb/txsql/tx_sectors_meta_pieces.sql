CREATE TABLE tx_car_pieces (
                                     piece_cid TEXT NOT NULL,
                                     car_key TEXT NOT NULL,
                                     piece_size BIGINT NOT NULL, -- padded size
                                     car_size BIGINT NOT NULL, -- raw size

                                     PRIMARY KEY (piece_cid)
);