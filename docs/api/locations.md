# API Reference

Packages:

- [locations.miloapis.com/v1alpha1](#locationsmiloapiscomv1alpha1)

# locations.miloapis.com/v1alpha1

Resource Types:

- [LocationClass](#locationclass)

- [Location](#location)

- [ServingLocation](#servinglocation)




## LocationClass
<sup><sup>[↩ Parent](#locationsmiloapiscomv1alpha1 )</sup></sup>






LocationClass is a kind of location that can be offered.

A Location says where; its class says what backs it. The class names the
controller that brings locations of that kind up, and points at the
configuration for the capacity behind them. Two locations in the same city
with different classes are different products.

A class lives in the control plane of whoever owns the capacity it
describes. Classes for the provider's own footprint live in the provider's
control plane: you name them from a Location, read the Accepted condition to
know they are usable, and do not create or edit them. If you are bringing
your own capacity, such as your own cloud account, you declare the class in
your control plane and it is yours to manage. Which control plane holds a
class is therefore the answer to whose capacity it is, and
spec.controllerName is the answer to who operates it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>locations.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>LocationClass</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#locationclassspec">spec</a></b></td>
        <td>object</td>
        <td>
          LocationClassSpec describes a kind of location the platform can offer.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#locationclassstatus">status</a></b></td>
        <td>object</td>
        <td>
          LocationClassStatus reports what the controller behind a class has made of
it.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### LocationClass.spec
<sup><sup>[↩ Parent](#locationclass)</sup></sup>



LocationClassSpec describes a kind of location the platform can offer.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>controllerName</b></td>
        <td>string</td>
        <td>
          ControllerName is the controller that reconciles Locations of this class.
It reads as a domain-qualified path, for example
`locations.miloapis.com/shared-edge`.

Only the controller named here acts on a Location of this class. Every
other controller ignores it, so two providers can serve locations side by
side in the same control plane without contending for the same objects.

You cannot change this field after creation. Retarget a class by creating
a new one and moving Locations across.<br/>
          <br/>
            <i>Validations</i>:<li>self == oldSelf: controllerName is immutable</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#locationclassspecparametersref">parametersRef</a></b></td>
        <td>object</td>
        <td>
          ParametersRef points at a resource holding the provider's own
configuration for this class: which capacity backs it, and whatever else
that provider needs to stand a location up. The shape of that resource is
the provider's to define, and this API makes no claim about it.

The resource is owned by the provider, not by you. Read it to understand
what a class offers; expect writes to be rejected.

Leave it unset for a class that needs no configuration. If it is set and
the resource cannot be read, or does not carry what the controller needs,
the class reports Accepted=False with reason InvalidParameters and no
Location of this class comes up.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### LocationClass.spec.parametersRef
<sup><sup>[↩ Parent](#locationclassspec)</sup></sup>



ParametersRef points at a resource holding the provider's own
configuration for this class: which capacity backs it, and whatever else
that provider needs to stand a location up. The shape of that resource is
the provider's to define, and this API makes no claim about it.

The resource is owned by the provider, not by you. Read it to understand
what a class offers; expect writes to be rejected.

Leave it unset for a class that needs no configuration. If it is set and
the resource cannot be read, or does not carry what the controller needs,
the class reports Accepted=False with reason InvalidParameters and no
Location of this class comes up.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>group</b></td>
        <td>string</td>
        <td>
          Group is the API group of the referent, for example
`compute.miloapis.com`. Use the empty string for the core API group.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          Kind is the kind of the referent, for example `EdgeCapacityPool`.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the referent.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### LocationClass.status
<sup><sup>[↩ Parent](#locationclass)</sup></sup>



LocationClassStatus reports what the controller behind a class has made of
it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#locationclassstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions describe the current state of the class.

`Accepted` tells you whether the controller named in spec.controllerName
has taken the class on. Until it is True, Locations of this class do
nothing. A class no controller recognises stays Accepted=Unknown with
reason Pending, which is what you see when the class names a controller
that is not running.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### LocationClass.status.conditions[index]
<sup><sup>[↩ Parent](#locationclassstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Location
<sup><sup>[↩ Parent](#locationsmiloapiscomv1alpha1 )</sup></sup>






Location is a place the platform serves traffic or runs workloads from,
typically a city rather than a cloud vendor's region.

Every Location names a LocationClass, and where that class lives tells you
whose capacity is behind the location. A class in the provider's control
plane is the provider's own footprint, offered to you: the provider owns the
capacity and operates it. A class in your control plane is capacity you
brought, such as your own cloud account, and you operate it. Either way the
controller named on the class is the one that acts, so nothing about the
location has to restate who runs it.

Locations the provider offers are declared once on the platform control
plane and projected into your project's control plane, and onto each cell as
a ServingLocation telling it where it sits. A projected Location is the
statement that the location is offered to you; edit the platform copy, not
the projection. Locations you declare yourself, on your own class, are yours
to edit.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>locations.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Location</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#locationspec">spec</a></b></td>
        <td>object</td>
        <td>
          LocationSpec defines the desired state of Location.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#locationstatus">status</a></b></td>
        <td>object</td>
        <td>
          LocationStatus defines the observed state of Location.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Location.spec
<sup><sup>[↩ Parent](#location)</sup></sup>



LocationSpec defines the desired state of Location.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#locationspeclocationclassref">locationClassRef</a></b></td>
        <td>object</td>
        <td>
          LocationClassRef names the LocationClass backing this location. The class
decides which controller brings the location up and what capacity sits
behind it.

Leave the project qualifier empty to name a class in the same control
plane as this Location. Set it to name a class in the provider's control
plane, which is what you do to place a location on capacity the provider
operates.

The class must exist and report Accepted=True before the location becomes
Ready. Naming a class that does not exist leaves the location not Ready
rather than rejecting it, so a location can be declared ahead of the
class that will serve it.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>topology</b></td>
        <td>map[string]string</td>
        <td>
          The topology of the location

This may contain arbitrary topology keys. Some keys may be well known, such
as:
	- topology.datum.net/city-code<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#locationspeccoordinates">coordinates</a></b></td>
        <td>object</td>
        <td>
          The geographic coordinates of the location, used by consumers that need
to plot the location on a map.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Location.spec.locationClassRef
<sup><sup>[↩ Parent](#locationspec)</sup></sup>



LocationClassRef names the LocationClass backing this location. The class
decides which controller brings the location up and what capacity sits
behind it.

Leave the project qualifier empty to name a class in the same control
plane as this Location. Set it to name a class in the provider's control
plane, which is what you do to place a location on capacity the provider
operates.

The class must exist and report Accepted=True before the location becomes
Ready. Naming a class that does not exist leaves the location not Ready
rather than rejecting it, so a location can be declared ahead of the
class that will serve it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name is the name of the LocationClass.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>project</b></td>
        <td>string</td>
        <td>
          Project names the project whose control plane holds the class. Leave it
unset, or empty, for a class that lives alongside this Location.

Set it when you are consuming capacity somebody else operates: the class
stays in their control plane, where they own it, and your Location points
across at it. You do not get a copy of the class, and you cannot change
what it offers.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Location.spec.coordinates
<sup><sup>[↩ Parent](#locationspec)</sup></sup>



The geographic coordinates of the location, used by consumers that need
to plot the location on a map.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>latitude</b></td>
        <td>string</td>
        <td>
          Latitude in decimal degrees, in the range [-90, 90].<br/>
          <br/>
            <i>Validations</i>:<li>double(self) >= -90.0 && double(self) <= 90.0: latitude must be between -90 and 90</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>longitude</b></td>
        <td>string</td>
        <td>
          Longitude in decimal degrees, in the range [-180, 180].<br/>
          <br/>
            <i>Validations</i>:<li>double(self) >= -180.0 && double(self) <= 180.0: longitude must be between -180 and 180</li>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Location.status
<sup><sup>[↩ Parent](#location)</sup></sup>



LocationStatus defines the observed state of Location.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#locationstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Represents the observations of a location's current state.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Location.status.conditions[index]
<sup><sup>[↩ Parent](#locationstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## ServingLocation
<sup><sup>[↩ Parent](#locationsmiloapiscomv1alpha1 )</sup></sup>






ServingLocation tells a cell which location it serves.

A cell is a cluster that runs workloads at one physical location. It cannot
tell where it is on its own, so the platform delivers it a ServingLocation:
a read-only copy of a Location, carrying the name and topology of the place
the cell sits in. Everything the cell does that depends on where it is,
such as claiming network addresses, resolves through this object.

A ServingLocation takes the name of the Location it was copied from. Expect
exactly one on a cell. Two or more means more than one location has been
delivered to the same cell, and the cell refuses to guess between them.

This object is managed for you. Create and edit Locations on the platform
control plane; the copies follow.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>locations.miloapis.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ServingLocation</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#servinglocationspec">spec</a></b></td>
        <td>object</td>
        <td>
          ServingLocationSpec describes the location a cell serves.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ServingLocation.spec
<sup><sup>[↩ Parent](#servinglocation)</sup></sup>



ServingLocationSpec describes the location a cell serves.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>topology</b></td>
        <td>map[string]string</td>
        <td>
          Topology describes where in the world this location is. Workloads placed
at this location inherit it, and placement rules that ask for a city or a
region are answered from these keys.

The map holds arbitrary keys. Some keys are well known:

	topology.datum.net/city-code: IAD
	topology.datum.net/region: us-east-1

You must supply topology.datum.net/city-code, and it must not be empty.
A location with no city code cannot serve placement requests that name a
city, so the API rejects it. Any other key you set is carried through
unchanged and is available to workloads at this location.

This field copies the topology of the Location it was published from.
Edit the Location, not this copy.<br/>
          <br/>
            <i>Validations</i>:<li>'topology.datum.net/city-code' in self && self['topology.datum.net/city-code'] != '': topology must carry a non-empty topology.datum.net/city-code</li>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#servinglocationspecsource">source</a></b></td>
        <td>object</td>
        <td>
          Source identifies the Location this copy came from. Use it to tell how
current the copy is: compare it against the Location of the same name to
see whether an edit has reached this cell yet.

The publisher sets this field. Leave it alone.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ServingLocation.spec.source
<sup><sup>[↩ Parent](#servinglocationspec)</sup></sup>



Source identifies the Location this copy came from. Use it to tell how
current the copy is: compare it against the Location of the same name to
see whether an edit has reached this cell yet.

The publisher sets this field. Leave it alone.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>generation</b></td>
        <td>integer</td>
        <td>
          Generation is the metadata.generation of the Location this copy came
from. When it is lower than the Location's current generation, an edit
has not reached this cell yet.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>publishedAt</b></td>
        <td>string</td>
        <td>
          PublishedAt is when the content of this copy last changed. A copy that is
re-checked but not changed keeps its original timestamp, so an old
timestamp means the location has been stable, not that publishing has
stalled.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
