#!/bin/bash

# install gosh executables
cd gosh
go install ./cmd/liner
go install ./cmd/inpage


# installs all the executables in src/
cd ../src/doc
go install .
