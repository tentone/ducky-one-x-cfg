#!/bin/bash

# Install virtual environment support
sudo apt update
sudo apt install python3-venv python3-pip -y

# Create a virtual environment
python3 -m venv .venv

# Activate it
source .venv/bin/activate

# Install dependencies
python -m pip install playwright

# Install Chromium
python -m playwright install --with-deps chromium

# Download the web application
python mirror_webapp.py download