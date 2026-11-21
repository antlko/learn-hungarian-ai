"""Apply the new form-of parsing to a COPY of a pack: replace wiktionary rows of every form-of page."""
import sqlite3, sys, collections
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from pipeline.wiktionary import parse, FormOf
from pipeline.build import feats_to_str
pack, dump = sys.argv[1], Path(sys.argv[2])
c = sqlite3.connect(pack)
lem = {}; byw = {}
for i, l, u in c.execute("select id,lemma,upos from lemma order by id"):
    lem.setdefault((l, u), i); byw.setdefault(l, i)
pages = 0; res = 0; new = []
for x in parse(dump):
    if not isinstance(x, FormOf) or x.upos != "VERB": continue
    pages += 1
    lid = lem.get((x.lemma, x.upos)) or byw.get(x.lemma)
    if lid is None: continue
    res += 1
    for r in x.readings:
        new.append((x.form, lid, x.upos, feats_to_str(r)))
print("pages", pages, "resolved", res, "rows", len(new))
c.execute("create temp table fo(form,lemma_id,upos,feats)")
c.executemany("insert into fo values (?,?,?,?)", new)
c.execute("create index fo_i on fo(form,lemma_id,upos)")
# old rows = wiktionary rows with a form-of page and not equal to a new reading
lost = c.execute("""select w.form,w.lemma_id,w.upos,w.feats from wordform w
 where w.source='wiktionary' and exists(select 1 from fo where fo.form=w.form and fo.lemma_id=w.lemma_id and fo.upos=w.upos)
 and not exists(select 1 from fo where fo.form=w.form and fo.lemma_id=w.lemma_id and fo.upos=w.upos and fo.feats=w.feats)""").fetchall()
print("old rows replaced", len(lost))
c.execute("""delete from wordform where source='wiktionary' and exists(select 1 from fo where fo.form=wordform.form and fo.lemma_id=wordform.lemma_id and fo.upos=wordform.upos)
 and not exists(select 1 from fo where fo.form=wordform.form and fo.lemma_id=wordform.lemma_id and fo.upos=wordform.upos and fo.feats=wordform.feats)""")
c.execute("""insert into wordform select distinct fo.form,fo.lemma_id,fo.upos,fo.feats,'wiktionary','high' from fo
 where not exists(select 1 from wordform w where w.form=fo.form and w.lemma_id=fo.lemma_id and w.upos=fo.upos and w.feats=fo.feats and w.source='wiktionary')""")
c.commit()
import json
json.dump([l for l in lost], open(sys.argv[3], "w"), ensure_ascii=False)
print("total", c.execute("select count(*) from wordform").fetchone())
