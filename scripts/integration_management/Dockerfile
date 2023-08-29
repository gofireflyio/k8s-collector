FROM --platform=${TARGETPLATFORM:-linux/amd64} python:alpine3.8
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY src /src
WORKDIR /src

ENV PYTHONUNBUFFERED=1

CMD ["python3", "main.py"]
