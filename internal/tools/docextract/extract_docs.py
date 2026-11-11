#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Maintainer tool: build a path-keyed doc set (nsd.DocSet JSON) from
locally licensed IEC 61850 PDFs.

The output contains text extracted from the standards. It is written to
a git-ignored path (official/docs_gen.json.gz by default) and must never
be committed: see docs/licensing.md and internal/tools/docextract/README.md.

Usage (from the repository root; "make docs-extract SPECS_DIR=..."):

    python3 -I internal/tools/docextract/extract_docs.py \
        --specs ~/IEC61850 --catalog official/catalog_gen.json \
        --out official/docs_gen.json.gz --work .docs-work

Requirements: PyMuPDF (pymupdf) for the tables and poppler's pdftotext
for the layout text. The PDFs are read in place; intermediate dumps go
to --work (git-ignored).

The elements and their keys come from the committed catalog, so every
key names an element of an embedded namespace:

    LN:<class>  DO:<class>.<do>  CDC:<cdc>  DA:<cdc>.<da>  SDO:<cdc>.<sdo>
    SP:<cdc>.<sp>  CA:<type>  SDA:<type>.<member>  FC:<fc>  BT:<type>
    PC:<condition>  ABBR:<name part>
"""
import argparse
import collections
import glob
import gzip
import json
import os
import re
import subprocess
import sys

FORMAT = 'otfabric-docset/1'
VERSION = '1'

# Spec editions the extractor understands, by part; the file is found as
# "IEC 61850-<part>-<year>.pdf" in --specs.
SPECS = {'7-2': '2010', '7-3': '2010', '7-4': '2010', '7-410': '2012', '7-420': '2009'}
OWN = {'IEC 61850-7-4': '7-4', 'IEC 61850-7-410': '7-410', 'IEC 61850-7-420': '7-420'}

# Title kinds; every other kind fills the description (nsd.DocKinds).
TITLE_KINDS = {'LN', 'CDC', 'CA', 'ENUM', 'FC', 'BT', 'TRG'}


# ---------------------------------------------------------------- input

def spec_pdf(specs, part):
    pat = os.path.join(specs, 'IEC 61850-%s-%s.pdf' % (part, SPECS[part]))
    hits = sorted(glob.glob(pat))
    if not hits:
        sys.exit('docextract: %s not found' % pat)
    return hits[0]


def dump_tables(pdf, dst):
    """Tables (pymupdf find_tables) and page text of a PDF, cached."""
    if os.path.exists(dst):
        with open(dst, encoding='utf-8') as fh:
            return json.load(fh)
    import pymupdf  # imported here: only needed when the cache is cold
    doc = pymupdf.open(pdf)
    out = []
    for pno in range(doc.page_count):
        page = doc[pno]
        try:
            tabs = [t.extract() for t in page.find_tables().tables]
        except Exception:  # pylint: disable=broad-except
            tabs = []
        out.append({'page': pno, 'text': page.get_text(), 'tables': tabs})
    with open(dst, 'w', encoding='utf-8') as fh:
        json.dump(out, fh)
    return out


def layout_text(pdf, dst):
    """pdftotext -layout output of a PDF, cached."""
    if not os.path.exists(dst):
        subprocess.run(['pdftotext', '-layout', pdf, dst], check=True)
    with open(dst, encoding='utf-8', errors='replace') as fh:
        return fh.read()


def english(pages):
    # Bilingual documents restart with a French foreword.
    for p in pages[8:]:
        if 'AVANT-PROPOS' in p['text'] or 'SOMMAIRE' in p['text']:
            return pages[:p['page']]
    return pages


# ------------------------------------------------------------- cleaning

FIG_MARK = re.compile(r'IEC\s+\d+/\d+')
IDENT = re.compile(r'^[A-Za-z][A-Za-z0-9_]*$')
QUAL = re.compile(r'^([A-Za-z][A-Za-z0-9_]*)\s*\(([^)]*)\)$')
CDCNAME = re.compile(r'^[A-Z][A-Z0-9]{1,5}$')


def clean(t):
    if t is None:
        return ''
    lines = [l.strip() for l in t.split('\n')]
    out, run = [], 0
    for l in lines:
        if FIG_MARK.search(l):
            break
        run = run + 1 if len(l) < 18 else 0
        if run >= 3:  # a figure fragment: a run of short lines
            out = out[:len(out) - 2]
            break
        out.append(l)
    s = ''
    for l in out:
        if not l:
            continue
        if s.endswith('-') and l[:1].islower():
            s += l
        else:
            s = (s + ' ' + l) if s else l
    return re.sub(r'\s+', ' ', s).strip()


def nonempty(row):
    return [c for c in (clean(c) if c is not None else None for c in row) if c]


def class_title(row, suffix='class'):
    ne = nonempty(row)
    if len(ne) == 1:
        m = re.match(r'^(.+?) ' + suffix + r'$', ne[0])
        if m:
            return m.group(1)
    return None


# ---------------------------------------------------------------- index

class Index:
    def __init__(self):
        self.ln_do = {}      # (LN, DO) -> text
        self.do_sem = {}     # DO -> text
        self.ln_title = {}   # LN -> title
        self.cdc_title = {}
        self.da_sem = {}     # name -> text
        self.da_sem_q = {}   # (name, CDC) -> text
        self.ca_title = {}
        self.ca_sda = {}     # (type, member) -> text or None
        self.pres = {}
        self.fc = {}
        self.basic = {}
        self.abbr = {}


def scan_ln(ix, pages, layout):
    cur = None
    for p in pages:
        for t in p['tables']:
            for row in t:
                ct = class_title(row)
                if ct:
                    cur = ct.strip()
                    continue
                ne = nonempty(row)
                if not ne:
                    continue
                if len(ne) == 2 and IDENT.match(ne[0]) and len(ne[1]) > 3 and not CDCNAME.match(ne[1]):
                    if ne[0] not in ('Data', 'LNName') and ne[1] != 'Semantics':
                        ix.do_sem.setdefault(ne[0], ne[1])
                    continue
                if cur and len(ne) >= 3 and IDENT.match(ne[0]) and CDCNAME.match(ne[1]):
                    ix.ln_do.setdefault((cur, ne[0]), ne[2])
    full = '\n'.join(p['text'] for p in pages) + '\n' + layout
    for m in re.finditer(r'LN:\s*([^\n]+?)\s+Name:\s*([A-Z][A-Za-z0-9]{3}(?: LN)?)\b', full):
        ix.ln_title.setdefault(m.group(2).strip(), re.sub(r'\s+', ' ', m.group(1)).strip())


def scan_abbr(ix, layout, start_pat, end_pat):
    starts = [m.start() for m in re.finditer(start_pat, layout, re.M)]
    if not starts:
        return
    s = starts[1] if len(starts) > 1 else starts[0]  # skip the table of contents
    e = re.compile(end_pat, re.M).search(layout, s + 100)
    for line in layout[s:e.start() if e else s + 60000].split('\n'):
        parts = re.split(r'\s{2,}', line.strip())
        for i in range(0, len(parts) - 1, 2):
            term, desc = parts[i], parts[i + 1]
            if IDENT.match(term) and len(term) <= 8 and desc and desc[0].isalpha() and desc != 'Description':
                ix.abbr.setdefault(term, desc)


def scan_73(ix, pages, layout):
    cur = None
    for p in pages:
        for t in p['tables']:
            for row in t:
                ct = class_title(row)
                if ct:
                    cur = ct
                    continue
                ctd = class_title(row, 'type definition')
                if ctd:
                    cur = ('CA', ctd)
                    continue
                ne = nonempty(row)
                if not ne:
                    continue
                if len(ne) == 2 and ne[1] != 'Semantics':
                    m = QUAL.match(ne[0])
                    if m:
                        for q in re.split(r'[,\s]+', m.group(2)):
                            if q:
                                ix.da_sem_q[(m.group(1), q)] = ne[1]
                        ix.da_sem.setdefault(m.group(1), ne[1])
                        continue
                    if IDENT.match(ne[0]) and len(ne[1]) > 15:
                        if ne[0] in ('M', 'O') or re.match(r'^(PICS|GC|AC)_', ne[0]):
                            ix.pres.setdefault(ne[0], ne[1])
                        else:
                            ix.da_sem.setdefault(ne[0], ne[1])
                        continue
                if isinstance(cur, tuple) and IDENT.match(ne[0]) and len(ne) >= 2:
                    vals = [x for x in (clean(c) if c else '' for c in row) if x]
                    if len(vals) >= 3 and re.search(r'[a-z]{3}', vals[2]) and '|' not in vals[2]:
                        ix.ca_sda.setdefault((cur[1], ne[0]), vals[2])
    txt = layout[:layout.find("Domaine d'application")] if "Domaine d'application" in layout else layout
    lines = txt.split('\n')
    for i, line in enumerate(lines):
        m = re.match(r'^\s*\d+(?:\.\d+)+\s+(.+?)\s*\(([A-Z]{2,5})\)\s*(?:\.{3,}.*)?$', line)
        if m:
            ix.cdc_title.setdefault(m.group(2), m.group(1).strip())
            continue
        # A heading wrapped onto a second line: "7.4.4  Phase to ground ... of a three-phase" / "system (WYE)".
        m = re.match(r'^\s*([^\d\s][^()]{0,60}?)?\s*\(([A-Z]{2,5})\)\s*(?:\.{3,}.*)?$', line)
        h = re.match(r'^\s*\d+(?:\.\d+)+\s+([A-Z][^()]+?)\s*$', lines[i - 1]) if m and i > 0 else None
        if h:
            ix.cdc_title.setdefault(m.group(2), re.sub(r'\s+', ' ', h.group(1) + ' ' + (m.group(1) or '')).strip())
    for p in pages:
        heads = [(mm.start(), mm.group(1)) for mm in re.finditer(r'(?m)^\s*\d+\.\d+(?:\.\d+)?\s+([A-Z][^\n.]{3,60})$', p['text'])]
        for mm in re.finditer(r'(?m)^\s*([A-Za-z]+) type definition\s*$', p['text']):
            prev = [h for h in heads if h[0] < mm.start()]
            if prev:
                ix.ca_title.setdefault(mm.group(1), prev[-1][1].strip())


BASIC_RE = re.compile(r'^(BOOLEAN|INT\d+U?|FLOAT\d+|OCTET STRING|VISIBLE STRING|UNICODE STRING|ENUMERATED|CODED ENUM|'
                      r'PACKED LIST|PACKED|TimeStamp|Quality|EntryTime|ObjectReference|Currency|PHYCOMADDR|TrgOps|'
                      r'OptFlds|SvOptFlds|Check|Dbpos|Tcmd)\b')


def scan_72(ix, pages):
    for p in pages:
        for t in p['tables']:
            for row in t:
                ne = nonempty(row)
                if not ne:
                    continue
                if len(ne) >= 3 and re.match(r'^[A-Z]{2}$', ne[0]) and re.search(r'[a-z]', ne[1]) and 'DataAttribute' in ' '.join(ne[2:]):
                    ix.fc.setdefault(ne[0], ne[1])
                if 2 <= len(ne) <= 3 and BASIC_RE.match(ne[0]):
                    ix.basic.setdefault(ne[0], ne[1])


# OTFabric-curated alias tables: NSD (Edition 2.1) names -> Edition 2 names.
PRES_ALIAS = {
    'M': 'M', 'O': 'O', 'MFsubst': 'PICS_SUBST', 'AtLeastOne': 'GC_1', 'AllOrNonePerGroup': 'GC_2_n',
    'AtMostOne': 'GC_1_EXCL', 'AllOnlyOneGroup': 'GC_2_XOR_n', 'MOln0': 'AC_LN0_M', 'MFln0': 'AC_LN0_EX',
    'MOlnNs': 'AC_DLD_M', 'MOdataNs': 'AC_DLN_M', 'MFscaledAV': 'AC_SCAV', 'MOrms': 'AC_RMS_M',
    'MOsbo': 'AC_CO_SBO', 'MAllOrNonePerGroup': 'AC_ST',
}
BASIC_ALIAS = {'VisString64': 'VISIBLE STRING', 'VisString129': 'VISIBLE STRING', 'VisString255': 'VISIBLE STRING',
               'Octet64': 'OCTET STRING', 'Unicode255': 'UNICODE STRING', 'Timestamp': 'TimeStamp',
               'ObjRef': 'ObjectReference', 'PhyComAddr': 'PHYCOMADDR'}


def build_index(specs, work):
    os.makedirs(work, exist_ok=True)
    ix = {d: Index() for d in SPECS}
    pages, layouts = {}, {}
    for part in SPECS:
        pdf = spec_pdf(specs, part)
        stem = os.path.join(work, os.path.splitext(os.path.basename(pdf))[0])
        pages[part] = english(dump_tables(pdf, stem + '.tables.json'))
        layouts[part] = layout_text(pdf, stem + '.txt')
    for part in ('7-4', '7-410', '7-420'):
        scan_ln(ix[part], pages[part], layouts[part])
    scan_abbr(ix['7-4'], layouts['7-4'], r'^4\s+Abbreviated terms', r'^5\s+Logical node classes')
    scan_abbr(ix['7-410'], layouts['7-410'], r'^4\s+Abbreviated terms', r'^5\s+')
    scan_abbr(ix['7-420'], layouts['7-420'], r'^4\.?\d*\s+Abbreviated terms|^4\s+Abbreviations', r'^5\s+')
    scan_73(ix['7-3'], pages['7-3'], layouts['7-3'])
    scan_72(ix['7-2'], pages['7-2'])
    return ix


# -------------------------------------------------------------- mapping

CONF_RANK = {'high': 3, 'medium': 2, 'low': 1}


class Builder:
    def __init__(self, ix, namespaces):
        self.ix = ix
        self.texts = {}   # key -> (text, conf, src)
        self.children = collections.defaultdict(set)
        base = {}
        for ns in namespaces:
            for ln in ns.get('abstractLNClasses', []) + ns.get('lnClasses', []):
                if ln.get('base'):
                    base[ln['name']] = ln['base']
        for c, b in base.items():
            seen = set()
            while b and b not in seen:
                seen.add(b)
                self.children[b].add(c)
                b = base.get(b)

    def put(self, key, text, conf, src):
        if not text:
            return
        old = self.texts.get(key)
        if old is None or CONF_RANK[conf] > CONF_RANK[old[1]]:
            self.texts[key] = (text, conf, src)

    def ln_indices(self, standard):
        own = OWN.get(standard)
        return [self.ix[p] for p in ([own] if own else []) + (['7-4'] if own != '7-4' else [])]

    def namespace(self, ns):
        std = ns['id']['standard']
        lnix = self.ln_indices(std)
        i2, i3 = self.ix['7-2'], self.ix['7-3']
        for ln in ns.get('abstractLNClasses', []) + ns.get('lnClasses', []):
            name = ln['name']
            for x in lnix:
                if name in x.ln_title:
                    self.put('LN:' + name, x.ln_title[name], 'high' if x is lnix[0] else 'medium', 'LN heading')
                    break
            for do in ln.get('dataObjects', []):
                self.data_object(name, do['name'], lnix)
        for a in ns.get('abbreviations', []):
            for x in lnix + [self.ix['7-420'], self.ix['7-410']]:
                if a['name'] in x.abbr:
                    self.put('ABBR:' + a['name'], x.abbr[a['name']], 'high' if x is lnix[0] else 'medium', 'abbreviations')
                    break
        for c in ns.get('cdcs', []):
            cdc = c['name']
            if cdc in i3.cdc_title:
                self.put('CDC:' + cdc, i3.cdc_title[cdc], 'high', '7-3 heading')
            for kind, items in (('SDO', c.get('subDataObjects', [])), ('DA', c.get('dataAttributes', [])),
                                ('SP', c.get('serviceParameters', []))):
                for m in items:
                    name = m['name'].lstrip('_')
                    if (name, cdc) in i3.da_sem_q:
                        self.put('%s:%s.%s' % (kind, cdc, name), i3.da_sem_q[(name, cdc)], 'high', '7-3 Table 64')
                    elif name in i3.da_sem:
                        self.put('%s:%s.%s' % (kind, cdc, name), i3.da_sem[name], 'medium', '7-3 Table 64')
        for ca in ns.get('constructedAttributes', []):
            n = ca['name']
            if n in i3.ca_title:
                self.put('CA:' + n, i3.ca_title[n], 'medium', '7-3 clause 6')
            for m in ca.get('subDataAttributes', []):
                v = i3.ca_sda.get((n, m['name']))
                if v:
                    self.put('SDA:%s.%s' % (n, m['name']), v, 'medium', '7-3 clause 6')
                elif m['name'] in i3.da_sem:
                    self.put('SDA:%s.%s' % (n, m['name']), i3.da_sem[m['name']], 'low', '7-3 Table 64')
        for fc in ns.get('functionalConstraints', []):
            a = fc['abbreviation']
            if a in i2.fc:
                self.put('FC:' + a, i2.fc[a], 'high', '7-2 Table 20')
        for bt in ns.get('basicTypes', []):
            n = BASIC_ALIAS.get(bt['name'], bt['name'])
            for b, v in i2.basic.items():
                if b == n or b.split()[0] == n:
                    self.put('BT:' + bt['name'], '%s: %s' % (b, v), 'medium', '7-2 Table 2')
                    break
        for pc in ns.get('presenceConditions', []):
            a = PRES_ALIAS.get(pc['name'])
            if a and a in i3.pres:
                self.put('PC:' + pc['name'], i3.pres[a], 'high' if a in ('M', 'O') else 'medium', '7-3 Table 1')

    def data_object(self, cls, member, lnix):
        key = 'DO:%s.%s' % (cls, member)
        for m in (member, member + '1'):  # Edition 2 names multi-instance classes with a trailing 1
            for x in lnix:
                if (cls, m) in x.ln_do:
                    self.put(key, x.ln_do[(cls, m)], 'high', 'LN table')
                    return
        cands = collections.Counter()
        for c in self.children.get(cls, ()):
            for x in lnix:
                for m in (member, member + '1'):
                    if (c, m) in x.ln_do:
                        cands[x.ln_do[(c, m)]] += 1
        for x in lnix:
            if ('Common LN', member) in x.ln_do:
                cands[x.ln_do[('Common LN', member)]] += 3
        if cands:
            self.put(key, cands.most_common(1)[0][0], 'medium', 'subclass LN tables')
            return
        for x in lnix:
            if member in x.do_sem:
                self.put(key, x.do_sem[member], 'low', 'DO semantics')
                return


def main():
    ap = argparse.ArgumentParser(description=__doc__.split('\n\n')[0])
    ap.add_argument('--specs', required=True, help='directory with the IEC 61850 PDFs')
    ap.add_argument('--catalog', default='official/catalog_gen.json')
    ap.add_argument('--out', default='official/docs_gen.json.gz')
    ap.add_argument('--work', default='.docs-work', help='cache of PDF dumps (git-ignored)')
    ap.add_argument('--report', default='', help='coverage report (default: <work>/report.txt)')
    ap.add_argument('--min-conf', default='low', choices=sorted(CONF_RANK))
    args = ap.parse_args()

    with open(args.catalog, encoding='utf-8') as fh:
        namespaces = json.load(fh)['namespaces']
    ix = build_index(args.specs, args.work)
    b = Builder(ix, namespaces)
    for ns in namespaces:
        b.namespace(ns)
    floor = CONF_RANK[args.min_conf]
    texts = {}
    for key, (text, conf, _) in sorted(b.texts.items()):
        if CONF_RANK[conf] < floor:
            continue
        field = 'title' if key.split(':', 1)[0] in TITLE_KINDS else 'description'
        texts[key] = {field: text}
    doc = {'format': FORMAT, 'sets': [{
        'lang': 'en',
        'source': 'docextract %s, local build from IEC 61850-7-2/7-3/7-4:2010, 7-410:2012, 7-420:2009; not redistributable' % VERSION,
        'texts': texts,
    }]}
    data = json.dumps(doc, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode('utf-8')
    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, 'wb') as fh:
        with gzip.GzipFile(fileobj=fh, mode='wb', mtime=0, filename='') as gz:
            gz.write(data)
    conf = collections.Counter(c for _, c, _ in b.texts.values())
    kinds = collections.Counter(k.split(':', 1)[0] for k in texts)
    report = args.report or os.path.join(args.work, 'report.txt')
    with open(report, 'w', encoding='utf-8') as fh:
        print('keys', len(texts), 'by kind', dict(sorted(kinds.items())), 'by confidence', dict(conf), file=fh)
        for key, (_, c, src) in sorted(b.texts.items()):
            print(key, c, src, sep='\t', file=fh)
    print('docextract: %d keys -> %s (report %s)' % (len(texts), args.out, report))


if __name__ == '__main__':
    main()
