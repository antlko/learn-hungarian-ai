# The model is converted at BUILD time and copied into a slim runtime stage, so
# the container starts ready. The alternative — converting on first boot — makes
# `docker compose up` look broken for the ten minutes it spends downloading
# 2.4 GB, with /health honestly but unhelpfully reporting "loading".
#
# torch is needed for the conversion and never again; leaving it out of the
# final image is worth about 800 MB.

FROM python:3.12-slim AS convert

ENV PIP_NO_CACHE_DIR=1
WORKDIR /build
COPY requirements.txt requirements-convert.txt ./
# torch's own dependencies come from PyPI: resolving them against the PyTorch-only
# index finds no usable wheels and falls back to building sdists (and fails).
# The managed install (internal/components) does the same two steps.
RUN pip install --no-deps --index-url https://download.pytorch.org/whl/cpu torch==2.5.1 \
 && pip install filelock "typing-extensions>=4.8.0" sympy==1.13.1 networkx jinja2 fsspec setuptools \
 && pip install -r requirements.txt

ARG MT_MODEL=facebook/nllb-200-distilled-600M
ARG MT_QUANTIZATION=int8
ENV MT_MODEL=${MT_MODEL} MT_QUANTIZATION=${MT_QUANTIZATION} MT_MODEL_DIR=/models/nllb-ct2
COPY app.py .
RUN python -c "import app; app.prepare_model()"

FROM python:3.12-slim

ENV PIP_NO_CACHE_DIR=1
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY --from=convert /models /models
COPY app.py .

ENV MT_MODEL_DIR=/models/nllb-ct2 MT_PORT=5003
EXPOSE 5003

# No healthcheck directive: the Go core probes GET /health itself and renders
# the result as a capability dot, which is where a learner will actually look.
CMD ["python", "app.py"]
