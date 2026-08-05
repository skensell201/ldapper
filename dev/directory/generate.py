#!/usr/bin/env python3
"""Generate a directory worth exploring.

The integration fixture in test/integration is deliberately small and boring:
it exists so a test can assert an exact number. This one is the opposite — a
plausible company with the awkward cases a real directory accumulates, for
driving Ldapper by hand and finding what no unit test predicted.

    python3 dev/directory/generate.py > dev/directory/seed.ldif
"""

import base64
import random
import sys

BASE = "dc=example,dc=com"
random.seed(20260805)  # a fixed seed, so two runs produce the same directory

FIRST = [
    "Anna", "Boris", "Daria", "Egor", "Irina", "Kirill", "Marina", "Nikita",
    "Olga", "Pavel", "Regina", "Sergey", "Tatiana", "Vadim", "Yulia", "Zakhar",
    "Alice", "Bob", "Carol", "David", "Emma", "Frank", "Grace", "Henry",
    "Ines", "Jonas", "Klara", "Lukas", "Maja", "Noah", "Oskar", "Petra",
]
LAST = [
    "Volkova", "Ivanov", "Orlova", "Titov", "Belova", "Zaytsev", "Rud",
    "Frolov", "Sokolova", "Morozov", "Novak", "Fischer", "Weber", "Meyer",
    "Schulz", "Becker", "Hoffmann", "Wagner", "Koch", "Richter",
]
DEPARTMENTS = ["Engineering", "Finance", "Sales", "Support", "Operations"]
TITLES = [
    "Systems Engineer", "Accountant", "Account Manager", "Support Analyst",
    "Site Reliability Engineer", "Controller", "Team Lead", "Intern",
]
CITIES = ["Berlin", "Prague", "Riga", "Lisbon", "Tallinn"]


def emit(dn, **attrs):
    """Write one entry. A list value becomes several lines of that attribute."""
    lines = [f"dn: {dn}"]
    for name, value in attrs.items():
        name = name.rstrip("_")  # class_ -> class
        for v in value if isinstance(value, list) else [value]:
            lines.append(encode(name, str(v)))
    print("\n".join(lines))
    print()


def encode(name, value):
    """LDIF needs base64 for anything that is not printable ASCII, and for
    values whose first character would change how the line parses."""
    if value and (
        not value.isascii()
        or not value.isprintable()
        or value[0] in " :<"
        or value.endswith(" ")
    ):
        return f"{name}:: {base64.b64encode(value.encode()).decode()}"
    return f"{name}: {value}"


def ou(dn, name, description=None):
    attrs = {"objectClass": ["top", "organizationalUnit"], "ou": name}
    if description:
        attrs["description"] = description
    emit(dn, **attrs)


def main():
    # ---- the root and the skeleton -------------------------------------
    emit(
        BASE,
        objectClass=["top", "dcObject", "organization"],
        o="Example Corporation",
        dc="example",
        description="A directory generated for exploring Ldapper by hand.",
    )

    ou(f"ou=corp,{BASE}", "corp", "Everything belonging to the company itself")
    ou(f"ou=computers,{BASE}", "computers", "Workstations and servers")
    ou(f"ou=system,{BASE}", "system", "Objects nobody should need to look at")

    people = f"ou=people,ou=corp,{BASE}"
    groups = f"ou=groups,ou=corp,{BASE}"
    services = f"ou=service accounts,ou=corp,{BASE}"
    departments = f"ou=departments,ou=corp,{BASE}"

    ou(people, "people", "Every person in the company")
    ou(groups, "groups", "Access groups")
    ou(services, "service accounts", "Accounts belonging to software, not people")
    ou(departments, "departments", "One branch per department")

    for d in DEPARTMENTS:
        ou(f"ou={d.lower()},{departments}", d.lower(), f"The {d} department")

    # ---- the awkward cases, first so they are easy to find --------------
    # A comma inside a name, which has to stay escaped everywhere.
    emit(
        f"cn=Volkova\\, Anna,{people}",
        objectClass=["top", "person", "organizationalPerson", "inetOrgPerson"],
        cn="Volkova, Anna",
        sn="Volkova",
        givenName="Anna",
        uid="a.volkova",
        mail=["a.volkova@example.com", "anna@example.com"],
        title="Systems Engineer",
        departmentNumber="Engineering",
        telephoneNumber="+49 30 123456",
        description="Her name is stored with a comma in it, which is legal and awkward.",
    )

    # Non-ASCII everywhere: the name, the description, the address.
    emit(
        f"cn=Анна Волкова,{people}",
        objectClass=["top", "person", "organizationalPerson", "inetOrgPerson"],
        cn="Анна Волкова",
        sn="Волкова",
        givenName="Анна",
        uid="a.volkova.ru",
        mail="a.volkova.ru@example.com",
        title="Инженер",
        description="Кириллица в каждом поле — проверка кодирования в LDIF и CSV.",
    )

    # Semicolons in a value, which is what the CSV export joins on.
    emit(
        f"cn=Semi Colon,{people}",
        objectClass=["top", "person", "organizationalPerson", "inetOrgPerson"],
        cn="Semi Colon",
        sn="Colon",
        uid="s.colon",
        description="A value with; several; semicolons in it.",
        mail=["one@example.com", "two@example.com", "three@example.com"],
    )

    # A value long enough to need wrapping in anything that wraps.
    emit(
        f"cn=Long Description,{people}",
        objectClass=["top", "person", "organizationalPerson", "inetOrgPerson"],
        cn="Long Description",
        sn="Description",
        uid="l.description",
        description="x" * 900,
    )

    # No mail at all, which one of the built-in filters looks for.
    emit(
        f"cn=No Mail,{people}",
        objectClass=["top", "person", "organizationalPerson", "inetOrgPerson"],
        cn="No Mail",
        sn="Mail",
        uid="n.mail",
        description="Deliberately carries no mail attribute.",
    )

    # POSIX attributes, so the POSIX dialect has something to find.
    for i, name in enumerate(["unix.one", "unix.two", "unix.three"], start=1):
        emit(
            f"cn={name},{people}",
            objectClass=["top", "person", "organizationalPerson", "inetOrgPerson", "posixAccount"],
            cn=name,
            sn=name.split(".")[1],
            uid=name,
            uidNumber=str(10000 + i),
            gidNumber="10000",
            homeDirectory=f"/home/{name}",
            loginShell="/bin/bash",
            mail=f"{name}@example.com",
        )

    # ---- the bulk of the people ----------------------------------------
    # Enough to need three pages at a page size of 1000, which is what
    # Active Directory defaults to.
    members = []
    total = 2500
    for i in range(1, total + 1):
        first = random.choice(FIRST)
        last = random.choice(LAST)
        uid = f"{first[0].lower()}.{last.lower()}{i}"
        dn = f"cn={first} {last} {i},{people}"
        members.append(dn)

        attrs = {
            "objectClass": ["top", "person", "organizationalPerson", "inetOrgPerson"],
            "cn": f"{first} {last} {i}",
            "sn": last,
            "givenName": first,
            "uid": uid,
            "employeeNumber": str(100000 + i),
            "title": random.choice(TITLES),
            "departmentNumber": random.choice(DEPARTMENTS),
            "l": random.choice(CITIES),
        }
        # One in eleven has no mail, so the filter for that finds a real set.
        if i % 11 != 0:
            attrs["mail"] = f"{uid}@example.com"
        # One in seven carries a telephone number.
        if i % 7 == 0:
            attrs["telephoneNumber"] = f"+49 30 {100000 + i}"
        emit(dn, **attrs)

    # ---- groups ---------------------------------------------------------
    emit(
        f"cn=all staff,{groups}",
        objectClass=["top", "groupOfNames"],
        cn="all staff",
        description="Everybody, which makes this the group worth paging through.",
        member=members[:500],
    )
    emit(
        f"cn=vpn users,{groups}",
        objectClass=["top", "groupOfNames"],
        cn="vpn users",
        description="Allowed to connect from outside.",
        member=members[:120],
    )
    emit(
        f"cn=administrators,{groups}",
        objectClass=["top", "groupOfNames"],
        cn="administrators",
        description="A small group, for comparing against a large one.",
        member=members[:3],
    )
    # posixGroup has no required member attribute, so this one is genuinely
    # empty — which is what the "empty groups" filter is looking for.
    emit(
        f"cn=empty group,{groups}",
        objectClass=["top", "posixGroup"],
        cn="empty group",
        gidNumber="20000",
        description="Has no members at all.",
    )
    for i, d in enumerate(DEPARTMENTS):
        emit(
            f"cn={d.lower()} team,{groups}",
            objectClass=["top", "groupOfNames"],
            cn=f"{d.lower()} team",
            description=f"Everyone in {d}.",
            member=members[i * 40 : (i + 1) * 40],
        )

    # ---- service accounts ----------------------------------------------
    for name, purpose in [
        ("svc-backup", "Runs the nightly backup"),
        ("svc-monitoring", "Reads the directory for the monitoring system"),
        ("svc-jenkins", "Continuous integration"),
        ("svc-ldapsync", "Replicates to the secondary directory"),
    ]:
        emit(
            f"cn={name},{services}",
            objectClass=["top", "person", "organizationalPerson", "inetOrgPerson"],
            cn=name,
            sn=name,
            uid=name,
            description=purpose,
        )

    # ---- computers ------------------------------------------------------
    for i in range(1, 61):
        emit(
            f"cn=ws-{i:03d},ou=computers,{BASE}",
            objectClass=["top", "device"],
            cn=f"ws-{i:03d}",
            description=f"Workstation {i} in {random.choice(CITIES)}",
            serialNumber=f"SN{200000 + i}",
        )

    print(f"# {total + 60 + 12} entries", file=sys.stderr)


if __name__ == "__main__":
    main()
