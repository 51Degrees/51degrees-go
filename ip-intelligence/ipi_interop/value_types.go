/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */
package ipi_interop

//#include <string.h>
//#include "ip-intelligence-cxx.h"
import "C"

type PropertyValueType int

// https://github.com/51Degrees/common-cxx/blob/version/4.5/propertyValueType.h#L53
const (
	StringValueType      PropertyValueType = iota // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_STRING
	IntegerValueType                              //FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_INTEGER
	DoubleValueType                               //FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_DOUBLE
	BooleanValueType                              // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_BOOLEAN
	JavascriptValueType                           // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_JAVASCRIPT
	FloatValueType                                // FIFTYONE_DEGREES_PROPERTY_VALUE_SINGLE_PRECISION_FLOAT
	ByteValueType                                 // FIFTYONE_DEGREES_PROPERTY_VALUE_SINGLE_BYTE
	CoordinateValueType                           // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_COORDINATE
	IpAddressValueType                            // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_IP_ADDRESS
	WkbValueType                                  // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WKB
	ObjectValueType                               // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_OBJECT
	DeclinationValueType                          // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_DECLINATION
	AzimuthValueType                              // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_AZIMUTH
	WkbRValueType                                 // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WKB_R
	WeightedStringValueType                       // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_STRING
	WeightedIntValueType                          // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_INT
	WeightedDoubleValueType                       // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_DOUBLE
	WeightedBoolValueType                         // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_BOOL
	WeightedSingleValueType                       // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_SINGLE
	WeightedByteValueType                         // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_BYTE
	WeightedIpAddressValueType                    // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_IP_ADDRESS
	WeightedWkbRValueType                         // FIFTYONE_DEGREES_PROPERTY_VALUE_TYPE_WEIGHTED_WKB_R
)

// IsWeighted reports whether the property value type is a weighted list type,
// meaning its values carry an intrinsic per-value confidence weight (e.g. Mcc,
// which is WeightedString). Non-weighted properties are single, deterministic
// values with no meaningful per-value weight.
func (t PropertyValueType) IsWeighted() bool {
	return t >= WeightedStringValueType && t <= WeightedWkbRValueType
}
