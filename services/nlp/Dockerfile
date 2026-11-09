# HuSpaCy sidecar. The model (hu_core_news_md, ~127 MB wheel, CC BY-SA 4.0) is
# deliberately NOT baked in: it is fetched on POST /warmup into the /models
# volume, so the image stays small and the weights survive container recreation.
# Not built or run in CI -- see README.md.
FROM python:3.12-slim

ENV PIP_NO_CACHE_DIR=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    OMP_NUM_THREADS=2 \
    OPENBLAS_NUM_THREADS=2 \
    MKL_NUM_THREADS=2 \
    NLP_MODEL_DIR=/models \
    NLP_PORT=5002

WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt \
 && useradd --create-home --uid 10001 nlp \
 && mkdir -p /models && chown nlp /models
COPY app.py .

USER nlp
VOLUME /models
EXPOSE 5002

# Liveness only (/livez): the model is an opt-in download, so a container that
# is waiting for /warmup is healthy. Readiness is GET /health (503 until ready).
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
  CMD python -c "import os,urllib.request as u; u.urlopen('http://127.0.0.1:%s/livez' % os.environ.get('NLP_PORT','5002'), timeout=4)" || exit 1

CMD ["python", "app.py"]
