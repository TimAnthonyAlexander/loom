#!/bin/bash

# Build the frontend assets
cd frontend
npm install
npm run build
cd ..

# Build the application
go build -o loomgui